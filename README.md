
# kured - Kubernetes Reboot Daemon

<img src="https://github.com/weaveworks/kured/raw/main/img/logo.png" align="right"/>

- [Introduction](#introduction)
- [Kubernetes & OS Compatibility](#kubernetes--os-compatibility)
- [Installation](#installation)
- [Configuration](#configuration)
  - [Reboot Sentinel File & Period](#reboot-sentinel-file--period)
  - [Setting a schedule](#setting-a-schedule)
  - [Blocking Reboots via Alerts](#blocking-reboots-via-alerts)
  - [Blocking Reboots via Pods](#blocking-reboots-via-pods)
  - [Prometheus Metrics](#prometheus-metrics)
  - [Notifications](#notifications)
  - [Overriding Lock Configuration](#overriding-lock-configuration)
- [Operation](#operation)
  - [Testing](#testing)
  - [Disabling Reboots](#disabling-reboots)
  - [Manual Unlock](#manual-unlock)
  - [Automatic Unlock](#automatic-unlock)
  - [Delaying Lock Release](#delaying-lock-release)
- [Building](#building)
- [Frequently Asked/Anticipated Questions](#frequently-askedanticipated-questions)
  - [Why is there no `latest` tag on Docker Hub?](#why-is-there-no-latest-tag-on-docker-hub)
- [Getting Help](#getting-help)

## Introduction

Kured (KUbernetes REboot Daemon) is a Kubernetes daemonset that
performs safe automatic node reboots when the need to do so is
indicated by the package management system of the underlying OS.

* Watches for the presence of a reboot sentinel file e.g. `/var/run/reboot-required`
  or the successful run of a sentinel command.
* Utilises a lock in the API server to ensure only one node reboots at
  a time
* Optionally defers reboots in the presence of active Prometheus alerts or selected pods
* Cordons & drains worker nodes before reboot, uncordoning them after

## Kubernetes & OS Compatibility

The daemon image contains versions of `k8s.io/client-go` and
`k8s.io/kubectl` (the binary of `kubectl` in older releases) for the purposes of
maintaining the lock and draining worker nodes. Kubernetes aims to provide
forwards and backwards compatibility of one minor version between client and
server:

| kured | kubectl | k8s.io/client-go | k8s.io/apimachinery | expected kubernetes compatibility |
|-------|---------|------------------|---------------------|-----------------------------------|
| main  | 1.21.4  | v0.21.4          | v0.21.4             | 1.20.x, 1.21.x, 1.22.x            |
| 1.7.0 | 1.20.5  | v0.20.5          | v0.20.5             | 1.19.x, 1.20.x, 1.21.x            |
| 1.6.1 | 1.19.4  | v0.19.4          | v0.19.4             | 1.18.x, 1.19.x, 1.20.x            |
| 1.5.1 | 1.18.8  | v0.18.8          | v0.18.8             | 1.17.x, 1.18.x, 1.19.x            |
| 1.4.4 | 1.17.7  | v0.17.0          | v0.17.0             | 1.16.x, 1.17.x, 1.18.x            |
| 1.3.0 | 1.15.10 | v12.0.0          | release-1.15        | 1.15.x, 1.16.x, 1.17.x            |
| 1.2.0 | 1.13.6  | v10.0.0          | release-1.13        | 1.12.x, 1.13.x, 1.14.x            |
| 1.1.0 | 1.12.1  | v9.0.0           | release-1.12        | 1.11.x, 1.12.x, 1.13.x            |
| 1.0.0 | 1.7.6   | v4.0.0           | release-1.7         | 1.6.x, 1.7.x, 1.8.x               |

See the [release notes](https://github.com/weaveworks/kured/releases)
for specific version compatibility information, including which
combination have been formally tested.

Versions >=1.1.0 enter the host mount namespace to invoke
`systemctl reboot`, so should work on any systemd distribution.

## Installation

To obtain a default installation without Prometheus alerting interlock
or Slack notifications:

```console
latest=$(curl -s https://api.github.com/repos/weaveworks/kured/releases | jq -r .[0].tag_name)
kubectl apply -f "https://github.com/weaveworks/kured/releases/download/$latest/kured-$latest-dockerhub.yaml"
```

If you want to customise the installation, download the manifest and
edit it in accordance with the following section before application.

## Configuration

The following arguments can be passed to kured via the daemonset pod template:

```console
Flags:
      --alert-filter-regexp regexp.Regexp   alert names to ignore when checking for active alerts
      --alert-firing-only bool              only consider firing alerts when checking for active alerts
      --blocking-pod-selector stringArray   label selector identifying pods whose presence should prevent reboots
      --drain-grace-period int              time in seconds given to each pod to terminate gracefully, if negative, the default value specified in the pod will be used (default: -1)
      --skip-wait-for-delete-timeout int    when seconds is greater than zero, skip waiting for the pods whose deletion timestamp is older than N seconds while draining a node (default: 0)
      --ds-name string                      name of daemonset on which to place lock (default "kured")
      --ds-namespace string                 namespace containing daemonset on which to place lock (default "kube-system")
      --end-time string                     schedule reboot only before this time of day (default "23:59:59")
      --force-reboot bool                   force a reboot even if the drain is still running (default: false)
      --drain-timeout duration              timeout after which the drain is aborted (default: 0, infinite time)
  -h, --help                                help for kured
      --lock-annotation string              annotation in which to record locking node (default "weave.works/kured-node-lock")
      --lock-release-delay duration         hold lock after reboot by this duration (default: 0, disabled)
      --lock-ttl duration                   expire lock annotation after this duration (default: 0, disabled)
      --message-template-drain string       message template used to notify about a node being drained (default "Draining node %s")
      --message-template-reboot string      message template used to notify about a node being rebooted (default "Rebooting node %s")
      --notify-url                          url for reboot notifications (cannot use with --slack-hook-url flags)
      --period duration                     reboot check period (default 1h0m0s)
      --prefer-no-schedule-taint string     Taint name applied during pending node reboot (to prevent receiving additional pods from other rebooting nodes). Disabled by default. Set e.g. to "weave.works/kured-node-reboot" to enable tainting.
      --prometheus-url string               Prometheus instance to probe for active alerts
      --reboot-command string               command to run when a reboot is required by the sentinel (default "/sbin/systemctl reboot")
      --reboot-days strings                 schedule reboot on these days (default [su,mo,tu,we,th,fr,sa])
      --reboot-sentinel string              path to file whose existence signals need to reboot (default "/var/run/reboot-required")
      --reboot-sentinel-command string      command for which a successful run signals need to reboot (default ""). If non-empty, sentinel file will be ignored.
      --slack-channel string                slack channel for reboot notfications
      --slack-hook-url string               slack hook URL for reboot notfications [deprecated in favor of --notify-url]
      --slack-username string               slack username for reboot notfications (default "kured")
      --start-time string                   schedule reboot only after this time of day (default "0:00")
      --time-zone string                    use this timezone for schedule inputs (default "UTC")
```

### Reboot Sentinel File & Period

By default kured checks for the existence of
`/var/run/reboot-required` every sixty minutes; you can override these
values with `--reboot-sentinel` and `--period`. Each replica of the
daemon uses a random offset derived from the period on startup so that
nodes don't all contend for the lock simultaneously.

Alternatively, a reboot sentinel command can be used. If a reboot
sentinel command is used, the reboot sentinel file presence will be
ignored.

### Setting a schedule

By default, kured will reboot any time it detects the sentinel, but this
may cause reboots during odd hours.  While service disruption does not
normally occur, anything is possible and operators may want to restrict
reboots to predictable schedules.  Use `--reboot-days`, `--start-time`,
`--end-time`, and `--time-zone` to set a schedule.  For example, business
hours on the west coast USA can be specified with:

```console
  --reboot-days=mon,tue,wed,thu,fri
  --start-time=9am
  --end-time=5pm
  --time-zone=America/Los_Angeles
```

Times can be formatted in numerous ways, including `5pm`, `5:00pm` `17:00`,
and `17`.  `--time-zone` represents a Go `time.Location`, and can be `UTC`,
`Local`, or any entry in the standard Linux tz database.

Note that when using smaller time windows, you should consider shortening
the sentinel check period (`--period`).

### Blocking Reboots via Alerts

You may find it desirable to block automatic node reboots when there
are active alerts - you can do so by providing the URL of your
Prometheus server:

```console
--prometheus-url=http://prometheus.monitoring.svc.cluster.local
```

By default the presence of *any* active (pending or firing) alerts
will block reboots, however you can ignore specific alerts:

```console
--alert-filter-regexp=^(RebootRequired|AnotherBenignAlert|...$
```

You can also only block reboots for firing alerts:
```console
--alert-firing-only=true
```

See the section on Prometheus metrics for an important application of this
filter.

### Blocking Reboots via Pods

You can also block reboots of an _individual node_ when specific pods
are scheduled on it:

```console
--blocking-pod-selector=runtime=long,cost=expensive
```

Since label selector strings use commas to express logical 'and', you can
specify this parameter multiple times for 'or':

```console
--blocking-pod-selector=runtime=long,cost=expensive
--blocking-pod-selector=name=temperamental
```

In this case, the presence of either an (appropriately labelled) expensive long
running job or a known temperamental pod on a node will stop it rebooting.

> Try not to abuse this mechanism - it's better to strive for
> restartability where possible. If you do use it, make sure you set
> up a RebootRequired alert as described in the next section so that
> you can intervene manually if reboots are blocked for too long.

### Prometheus Metrics

Each kured pod exposes a single gauge metric (`:8080/metrics`) that
indicates the presence of the sentinel file:

```console
# HELP kured_reboot_required OS requires reboot due to software updates.
# TYPE kured_reboot_required gauge
kured_reboot_required{node="ip-xxx-xxx-xxx-xxx.ec2.internal"} 0
```

The purpose of this metric is to power an alert which will summon an
operator if the cluster cannot reboot itself automatically for a
prolonged period:

```console
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

### Notifications

When you specify a formatted URL using `--notify-url`, kured will notify
about draining and rebooting nodes across a list of technologies.

![Notification](img/slack-notification.png)

Alternatively you can use the `--message-template-drain` and `--message-template-reboot` to customize the text of the message, e.g.

```cli
--message-template-drain="Draining node %s part of *my-cluster* in region *xyz*"
```

Here is the syntax:

- slack:           `slack://tokenA/tokenB/tokenC`
(`--slack-hook-url` is deprecated but possible to use)

- rocketchat:      `rocketchat://[username@]rocketchat-host/token[/channel|@recipient]`

- teams:           `teams://tName/token-a/token-b/token-c`

   > **Attention** as the [format of the url has changed](https://github.com/containrrr/shoutrrr/issues/138) you also have to specify a `tName`

- Email:           `smtp://username:password@host:port/?fromAddress=fromAddress&toAddresses=recipient1[,recipient2,...]`

More details here: [containrrr.dev/shoutrrr/v0.4/services/overview](https://containrrr.dev/shoutrrr/v0.4/services/overview)

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

```console
sudo touch /var/run/reboot-required
```

### Disabling Reboots

If you need to temporarily stop kured from rebooting any nodes, you
can take the lock manually:

```console
kubectl -n kube-system annotate ds kured weave.works/kured-node-lock='{"nodeID":"manual"}'
```

Don't forget to release it afterwards!

### Manual Unlock

In exceptional circumstances, such as a node experiencing a permanent
failure whilst rebooting, manual intervention may be required to
remove the cluster lock:

```console
kubectl -n kube-system annotate ds kured weave.works/kured-node-lock-
```

> NB the `-` at the end of the command is important - it instructs
> `kubectl` to remove that annotation entirely.

### Automatic Unlock

In exceptional circumstances (especially when used with cluster-autoscaler) a node
which holds lock might be killed thus annotation will stay there for ever.

Using `--lock-ttl=30m` will allow other nodes to take over if TTL has expired (in this case 30min) and continue reboot process.

### Delaying Lock Release


Using `--lock-release-delay=30m` will cause nodes to hold the lock for the specified time frame (in this case 30min) before it is released and the reboot process continues. This can be used to throttle reboots across the cluster.

## Building

Kured now uses [Go
Modules](https://github.com/golang/go/wiki/Modules), so build
instructions vary depending on where you have checked out the
repository:

**Building outside $GOPATH:**

```console
make
```

**Building inside $GOPATH:**

```console
GO111MODULE=on make
```

You can find the current preferred version of Golang in the [go.mod file](go.mod).

If you are interested in contributing code to kured, please take a look at
our [development][development] docs.

[development]: DEVELOPMENT.md

## Frequently Asked/Anticipated Questions

### Why is there no `latest` tag on Docker Hub?

Use of `latest` for production deployments is bad practice - see
[here](https://kubernetes.io/docs/concepts/configuration/overview) for
details. The manifest on `main` refers to `latest` for local
development testing with minikube only; for production use choose a
versioned manifest from the [release page](https://github.com/weaveworks/kured/releases/).

## Getting Help

If you have any questions about, feedback for or problems with `kured`:

* Invite yourself to the <a href="https://slack.weave.works/" target="_blank">Weave Users Slack</a>.
* Ask a question on the [#kured](https://weave-community.slack.com/messages/kured/) slack channel.
* [File an issue](https://github.com/weaveworks/kured/issues/new).
* Join us in [our monthly meeting](https://docs.google.com/document/d/1bsHTjHhqaaZ7yJnXF6W8c89UB_yn-OoSZEmDnIP34n8/edit#),
  every fourth Wednesday of the month at 16:00 UTC.

We follow the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md).

Your feedback is always welcome!
