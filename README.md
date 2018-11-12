
<img src="https://github.com/weaveworks/kured/raw/master/img/logo.png" align="right"/>

* [Introduction](#introduction)
* [Kubernetes & OS Compatibility](#kubernetes-&-os-compatibility)
* [Installation](#installation)
* [Configuration](#configuration)
	* [Reboot Sentinel File & Period](#reboot-sentinel-file-&-period)
	* [Blocking Reboots via Alerts](#blocking-reboots-via-alerts)
	* [Prometheus Metrics](#prometheus-metrics)
	* [Slack Notifications](#slack-notifications)
	* [Overriding Lock Configuration](#overriding-lock-configuration)
* [Operation](#operation)
	* [Testing](#testing)
	* [Disabling Reboots](#disabling-reboots)
	* [Manual Unlock](#manual-unlock)
* [Building](#building)
* [Frequently Asked/Anticipated Questions](#frequently-askedanticipated-questions)
* [Getting Help](#getting-help)

## Introduction

Kured (KUbernetes REboot Daemon) is a Kubernetes daemonset that
performs safe automatic node reboots when the need to do so is
indicated by the package management system of the underlying OS.

* Watches for the presence of a reboot sentinel e.g. `/var/run/reboot-required`
* Utilises a lock in the API server to ensure only one node reboots at
  a time
* Optionally defers reboots in the presence of active Prometheus alerts
* Cordons & drains worker nodes before reboot, uncordoning them after

## Kubernetes & OS Compatibility

The daemon image contains versions of `k8s.io/client-go` and the
`kubectl` binary for the purposes of maintaining the lock and draining
worker nodes. Kubernetes aims to provide forwards & backwards
compatibility of one minor version between client and server:

| kured  | kubectl | k8s.io/client-go | k8s.io/apimachinery | expected kubernetes compatibility |
|--------|---------|------------------|---------------------|-----------------------------------|
| 1.1.0  | 1.12.1  | v9.0.0           | release-1.12        | 1.11.x, 1.12.x, 1.13.x            |
| 1.0.0  | 1.7.6   | v4.0.0           | release-1.7         | 1.6.x, 1.7.x, 1.8.x               | 

See the [release notes](https://github.com/weaveworks/kured/releases)
for specific version compatibility information, including which
combination have been formally tested.

Versions >=1.1.0 enter the host mount namespace to invoke
`systemctl reboot`, so should work on any systemd distribution.

## Installation

To obtain a default installation without Prometheus alerting interlock
or Slack notifications:

```
kubectl apply -f https://github.com/weaveworks/kured/releases/download/1.1.0/kured-1.1.0.yaml
```

If you want to customise the installation, download the manifest and
edit it in accordance with the following section before application.

## Configuration

The following arguments can be passed to kured via the daemonset pod template:

```
Flags:
      --alert-filter-regexp value   alert names to ignore when checking for active alerts
      --ds-name string              namespace containing daemonset on which to place lock (default "kube-system")
      --ds-namespace string         name of daemonset on which to place lock (default "kured")
      --lock-annotation string      annotation in which to record locking node (default "weave.works/kured-node-lock")
      --period duration             reboot check period (default 1h0m0s)
      --prometheus-url string       Prometheus instance to probe for active alerts
      --reboot-sentinel string      path to file whose existence signals need to reboot (default "/var/run/reboot-required")
      --metric-only bool            do not reboot - expose just the metric (default false)
      --slack-hook-url string       slack hook URL for reboot notfications
      --slack-username string       slack username for reboot notfications (default "kured")
```

### Reboot Sentinel File & Period

By default kured checks for the existence of
`/var/run/reboot-required` every sixty minutes; you can override these
values with `--reboot-sentinel` and `--period`. Each replica of the
daemon uses a random offset derived from the period on startup so that
nodes don't all contend for the lock simultaneously.

### Blocking Reboots via Alerts

You may find it desirable to block automatic node reboots when there
are active alerts - you can do so by providing the URL of your
Prometheus server:

```
--prometheus-url=http://prometheus.monitoring.svc.cluster.local
```

By default the presence of *any* active (pending or firing) alerts
will block reboots, however you can ignore specific alerts:

```
--alert-filter-regexp=^(RebootRequired|AnotherBenignAlert|...$
```

An important application of this filter will become apparent in the
next section.

### Prometheus Metrics

Each kured pod exposes a single gauge metric (`:8080/metrics`) that
indicates the presence of the sentinel file:

```
# HELP kured_reboot_required OS requires reboot due to software updates.
# TYPE kured_reboot_required gauge
kured_reboot_required{node="ip-xxx-xxx-xxx-xxx.ec2.internal"} 0
```

The purpose of this metric is to power an alert which will summon an
operator if the cluster cannot reboot itself automatically for a
prolonged period:

```
# Alert if a reboot is required for any machines. Acts as a failsafe for the
# reboot daemon, which will not reboot nodes if there are pending alerts save
# this one.
ALERT RebootRequired
  IF          max(kured_reboot_required) != 0
  FOR         24h
  LABELS      { severity="warning" }
  ANNOTATIONS {
    summary = "Machine(s) require being rebooted, and the reboot daemon has failed to do so for 24 hours",
    impact = "Cluster nodes more vulnerable to security exploits. Eventually, no disk space left.",
    description = "Machine(s) require being rebooted, probably due to kernel update.",
  }
```

If you choose to employ such an alert and have configured kured to
probe for active alerts before rebooting, be sure to specify
`--alert-filter-regexp=^RebootRequired$` to avoid deadlock!

### Slack Notifications

If you specify a Slack hook via `--slack-hook-url`, kured will notify
you immediately prior to rebooting a node:

<img src="https://github.com/weaveworks/kured/raw/master/img/slack-notification.png"/>

We recommend setting `--slack-username` to be the name of the
environment, e.g. `dev` or `prod`.

### Overriding Lock Configuration

The `--ds-name` and `--ds-namespace` arguments should match the name and
namespace of the daemonset used to deploy the reboot daemon - the locking is
implemented by means of an annotation on this resource. The defaults match
the daemonset YAML provided in the repository.

Similarly `--lock-annotation` can be used to change the name of the
annotation kured will use to store the lock, but the default is almost
certainly safe.

## Operation

The example commands in this section assume that you have not
overriden the default lock annotation, daemonset name or namespace;
if you have, you will have to adjust the commands accordingly.

### Testing

You can test your configuration by provoking a reboot on a node:

```
sudo touch /var/run/reboot-required
```

### Disabling Reboots

If you need to temporarily stop kured from rebooting any nodes, you
can take the lock manually:

```
kubectl -n kube-system annotate ds kured weave.works/kured-node-lock='{"nodeID":"manual"}'
```

Don't forget to release it afterwards!

### Manual Unlock

In exceptional circumstances, such as a node experiencing a permanent
failure whilst rebooting, manual intervention may be required to
remove the cluster lock:

```
kubectl -n kube-system annotate ds kured weave.works/kured-node-lock-
```
> NB the `-` at the end of the command is important - it instructs
> `kubectl` to remove that annotation entirely.

## Building

```
dep ensure && make
```

## Frequently Asked/Anticipated Questions

### Why is there no `latest` tag on quay.io?

Use of `latest` for production deployments is bad practice - see
[here](https://kubernetes.io/docs/concepts/configuration/overview) for
details. The manifest on `master` refers to `latest` for local
development testing with minikube only; for production use choose a
versioned manifest from the [release page](https://github.com/weaveworks/kured/releases/).

## Getting Help

If you have any questions about, feedback for or problems with `kured`:

- Invite yourself to the <a href="https://weaveworks.github.io/community-slack/" target="_blank"> #weave-community </a> slack channel.
- Ask a question on the <a href="https://weave-community.slack.com/messages/general/"> #weave-community</a> slack channel.
- Send an email to <a href="mailto:weave-users@weave.works">weave-users@weave.works</a>
- <a href="https://github.com/weaveworks/kured/issues/new">File an issue.</a>

Your feedback is always welcome!
