# Kured (KUbernetes REboot Daemon)

## Introduction
This chart installs the "Kubernetes Reboot Daemon" using the Helm Package Manager.

## Prerequisites
- Kubernetes 1.9+

## Installing the Chart
To install the chart with the release name `my-release`:
```bash
$ helm repo add kured https://weaveworks.github.io/kured
$ helm install my-release kured/kured
```

## Uninstalling the Chart
To uninstall/delete the `my-release` deployment:
```bash
$ helm delete my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.


## Migrate from stable Helm-Chart
The following changes have been made compared to the stable chart:
- **[BREAKING CHANGE]** The `autolock` feature was removed. Use `configuration.startTime` and `configuration.endTime` instead.
- Role inconsistencies have been fixed (allowed verbs for modifying the `DaemonSet`, apiGroup of `PodSecurityPolicy`)
- Added support for affinities.
- Configuration of cli-flags can be made through a `configuration` object.
- Added optional `Service` and `ServiceMonitor` support for metrics endpoint.


## Configuration

| Config                  | Description                                                                 | Default                    |
| ------                  | -----------                                                                 | -------                    |
| `image.repository`      | Image repository                                                            | `weaveworks/kured` |
| `image.tag`             | Image tag                                                                   | `1.9.1`                    |
| `image.pullPolicy`      | Image pull policy                                                           | `IfNotPresent`             |
| `image.pullSecrets`     | Image pull secrets                                                          | `[]`                       |
| `updateStrategy`        | Daemonset update strategy                                                   | `RollingUpdate`            |
| `maxUnavailable`        | The max pods unavailable during a rolling update                            | `1`                        |
| `podAnnotations`        | Annotations to apply to pods (eg to add Prometheus annotations)             | `{}`                       |
| `dsAnnotations`         | Annotations to apply to the kured DaemonSet                                 | `{}`                       |
| `extraArgs`             | Extra arguments to pass to `/usr/bin/kured`. See below.                     | `{}`                       |
| `extraEnvVars`          | Array of environment variables to pass to the daemonset.                    | `{}`                       |
| `configuration.lockTtl` | cli-parameter `--lock-ttl`                                                  | `0`                       |
| `configuration.lockReleaseDelay` | cli-parameter `--lock-release-delay`                               | `0`                       |
| `configuration.alertFilterRegexp` | cli-parameter `--alert-filter-regexp`                             | `""`                       |
| `configuration.alertFiringOnly` | cli-parameter `--alert-firing-only`                                 | `false`                   |
| `configuration.blockingPodSelector` | Array of selectors for multiple cli-parameters `--blocking-pod-selector` | `[]`             |
| `configuration.endTime` | cli-parameter `--end-time`                                                  | `""`                      |
| `configuration.lockAnnotation` | cli-parameter `--lock-annotation`                                    | `""`                      |
| `configuration.period` | cli-parameter `--period`                                                     | `""`                      |
| `configuration.forceReboot` | cli-parameter `--force-reboot`                                          | `false`                   |
| `configuration.drainGracePeriod` | cli-parameter `--drain-grace-period`                               | `""`                      |
| `configuration.drainTimeout` | cli-parameter `--drain-timeout`                                        | `""`                      |
| `configuration.skipWaitForDeleteTimeout` | cli-parameter `--skip-wait-for-delete-timeout`             | `""`                      |
| `configuration.prometheusUrl` | cli-parameter `--prometheus-url`                                      | `""`                      |
| `configuration.rebootDays` | Array of days for multiple cli-parameters `--reboot-days`                | `[]`                      |
| `configuration.rebootSentinel` | cli-parameter `--reboot-sentinel`                                    | `""`                      |
| `configuration.rebootSentinelCommand` | cli-parameter `--reboot-sentinel-command`                     | `""`                      |
| `configuration.rebootCommand` | cli-parameter `--reboot-command`                                      | `""`                      |
| `configuration.rebootDelay` | cli-parameter `--reboot-delay`                                          | `""`                      |
| `configuration.slackChannel` | cli-parameter `--slack-channel`                                        | `""`                      |
| `configuration.slackHookUrl` | cli-parameter `--slack-hook-url`                                       | `""`                      |
| `configuration.slackUsername` | cli-parameter `--slack-username`                                      | `""`                      |
| `configuration.notifyUrl` | cli-parameter `--notify-url`                                              | `""`                      |
| `configuration.messageTemplateDrain` | cli-parameter `--message-template-drain`                       | `""`                      |
| `configuration.messageTemplateReboot` | cli-parameter `--message-template-reboot`                     | `""`                      |
| `configuration.startTime` | cli-parameter `--start-time`                                              | `""`                      |
| `configuration.timeZone` | cli-parameter `--time-zone`                                                | `""`                      |
| `configuration.annotateNodes` | cli-parameter `--annotate-nodes`                                      | `false`                   |
| `configuration.logFormat` | cli-parameter `--log-format`                                              | `"text"`                  |
| `configuration.preferNoScheduleTaint` | Taint name applied during pending node reboot                 | `""`                   |
| `rbac.create`           | Create RBAC roles                                                           | `true`                     |
| `serviceAccount.create` | Create a service account                                                    | `true`                     |
| `serviceAccount.name`   | Service account name to create (or use if `serviceAccount.create` is false) | (chart fullname)           |
| `podSecurityPolicy.create` | Create podSecurityPolicy                                                 | `false`                     |
| `resources`             | Resources requests and limits.                                              | `{}`                       |
| `metrics.create`        | Create a ServiceMonitor for prometheus-operator                             | `false`                    |
| `metrics.namespace`     | The namespace to create the ServiceMonitor in                               | `""`                    |
| `metrics.labels`        | Additional labels for the ServiceMonitor                                    | `{}`                    |
| `metrics.interval`      | Interval prometheus should scrape the endpoint                              | `60s`                   |
| `metrics.scrapeTimeout` | A custom scrapeTimeout for prometheus                                       | `""`                    |
| `service.create`        | Create a Service for the metrics endpoint                                   | `false`                    |
| `service.name  `        | Service name for the metrics endpoint                                       | `""`                       |
| `service.port`          | Port of the service to expose                                               | `8080`                     |
| `service.annotations`   | Annotations to apply to the service (eg to add Prometheus annotations)      | `{}`                       |
| `podLabels`             | Additional labels for pods (e.g. CostCenter=IT)                             | `{}`                       |
| `priorityClassName`     | Priority Class to be used by the pods                                       | `""`                       |
| `tolerations`           | Tolerations to apply to the daemonset (eg to allow running on master)       | `[{"key": "node-role.kubernetes.io/master", "effect": "NoSchedule"}]`|
| `affinity`              | Affinity for the daemonset (ie, restrict which nodes kured runs on)         | `{}`                       |
| `nodeSelector`          | Node Selector for the daemonset (ie, restrict which nodes kured runs on)    | `{}`                       |
| `volumeMounts`          | Maps of volumes mount to mount                                              | `{}`                       |
| `volumes`               | Maps of volumes to mount                                                    | `{}`                       |
See https://github.com/weaveworks/kured#configuration for values (not contained in the `configuration` object) for `extraArgs`. Note that
```yaml
extraArgs:
  foo: 1
  bar-baz: 2
```
becomes `/usr/bin/kured ... --foo=1 --bar-baz=2`.


## Prometheus Metrics

Kured exposes a single prometheus metric indicating whether a reboot is required or not (see [kured docs](https://github.com/weaveworks/kured#prometheus-metrics)) for details.

#### Prometheus-Operator

```yaml
metrics:
  create: true
```

#### Prometheus Annotations

```yaml
service:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/path: "/metrics"
    prometheus.io/port: "8080"
```
