# kured - Kubernetes Reboot Daemon

[![Artifact HUB](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/kured)](https://artifacthub.io/packages/helm/kured/kured)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fkubereboot%2Fkured.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fkubereboot%2Fkured?ref=badge_shield)
[![CLOMonitor](https://img.shields.io/endpoint?url=https://clomonitor.io/api/projects/cncf/kured/badge)](https://clomonitor.io/projects/cncf/kured)

<img src="https://github.com/kubereboot/website/raw/main/static/img/kured.png" alt="kured logo" width="200" align="right"/>

- [kured - Kubernetes Reboot Daemon](#kured---kubernetes-reboot-daemon)
  - [Introduction](#introduction)
  - [Documentation](#documentation)
  - [Getting Help](#getting-help)
  - [Trademarks](#trademarks)
  - [License](#license)

## Introduction

Kured (KUbernetes REboot Daemon) is a Kubernetes daemonset that
performs safe automatic node reboots when the need to do so is
indicated by the package management system of the underlying OS.

- Watches for the presence of a reboot sentinel file e.g. `/var/run/reboot-required`
  or the successful run of a sentinel command.
- Utilises a lock in the API server to ensure only one node reboots at
  a time
- Optionally defers reboots in the presence of active Prometheus alerts or selected pods
- Cordons & drains worker nodes before reboot, uncordoning them after

## Documentation

Find all our docs on <https://kured.dev>:

- [All Kured Documentation](https://kured.dev/docs/)
- [Installing Kured](https://kured.dev/docs/installation/)
- [Configuring Kured](https://kured.dev/docs/configuration/)
- [Operating Kured](https://kured.dev/docs/operation/)
- [Developing Kured](https://kured.dev/docs/development/)

And there's much more!

## Getting Help

If you have any questions about, feedback for or problems with `kured`:

- Invite yourself to the <a href="https://slack.cncf.io/" target="_blank">CNCF Slack</a>.
- Ask a question on the [#kured](https://cloud-native.slack.com/archives/kured) slack channel.
- [File an issue](https://github.com/kubereboot/kured/issues/new).
- Join us in [our monthly meeting](https://docs.google.com/document/d/1AWT8YDdqZY-Se6Y1oAlwtujWLVpNVK2M_F_Vfqw06aI/edit),
  every first Wednesday of the month at 16:00 UTC.
- You might want to [join the kured-dev mailing list](https://lists.cncf.io/g/cncf-kured-dev) as well.

We follow the [CNCF Code of Conduct](CODE_OF_CONDUCT.md).

Your feedback is always welcome!

## Trademarks

**Kured is a [Cloud Native Computing Foundation](https://cncf.io/) Sandbox project.**

![Cloud Native Computing Foundation logo](img/cncf-color.png)

The Linux Foundation® (TLF) has registered trademarks and uses trademarks. For a list of TLF trademarks, see [Trademark Usage](https://www.linuxfoundation.org/trademark-usage/).

## License

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fkubereboot%2Fkured.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fkubereboot%2Fkured?ref=badge_large)
