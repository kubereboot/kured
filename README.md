
* [Introduction](#introduction)
* [Configuration](#configuration)
	* [Reboot Sentinel File & Period](#reboot-sentinel-file-&-period)
	* [Blocking Reboots via Alerts](#blocking-reboots-via-alerts)
	* [Overriding Lock Configuration](#overriding-lock-configuration)
* [Building](#building)

## Introduction

Kured (KUbernetes REboot Daemon) is a Kubernetes daemonset that
performs safe automatic node reboots when it is requested by the
package management system of the underlying OS.

* Watches for the presence of a reboot sentinel e.g. `/var/run/reboot-required` 
* Utilises a lock in the API server to ensure only one node reboots at
  a time
* Optionally defers reboots in the presence of active Prometheus alerts
* Cordons & drains worker nodes before reboot, uncordoning them after

## Configuration

The following arguments can be passed to kured via the daemonset pod template:

```
Flags:
      --alert-filter-regexp value   alert names to ignore when checking for active alerts
      --ds-name string              namespace containing daemonset on which to place lock (default "kube-system")
      --ds-namespace string         name of daemonset on which to place lock (default "kured")
      --lock-annotation string      annotation in which to record locking node (default "weave.works/kured-node-lock")
      --period int                  reboot check period in minutes (default 60)
      --prometheus-url string       Prometheus instance to probe for active alerts
      --reboot-sentinel string      path to file whose existence signals need to reboot (default "/var/run/reboot-required")
```

### Reboot Sentinel File & Period

By default kured checks for the existence of
`/var/run/reboot-required` every sixty minutes; you can override these
values with `--reboot-sentinel` and `--period`. Each instance of the
reboot uses a random offset derived from the period on startup so that
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
--alert-filter-regexp=^(BenignAlert|AnotherBenignAlert|...$
```

### Overriding Lock Configuration

The `--ds-name` and `--ds-namespace` arguments should match the name and
namespace of the daemonset used to deploy the reboot daemon - the locking is
implemented by means of an annotation on this resource. The defaults match
the daemonset YAML provided in the repository.

Similarly `--lock-annotation` can be used to change the name of the
annotation kured will use to store the lock, but the default is almost
certainly safe.

## Building

```
dep ensure && make
```
