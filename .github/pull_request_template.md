## Description

<!-- Briefly describe the change and the problem it solves. -->

Fixes #

## Validation

<!-- Be specific. List the exact commands you ran (e.g. `make test`, `make e2e`,
     manual reboot run on a kind cluster) and what you observed. -->

- [ ] `make test` passes locally.
- [ ] If the change affects the reboot lifecycle, signal handling, or daemon
      startup, I exercised it on a real or kind cluster and verified the
      observed behaviour matches the description.

## User-facing changes

<!-- For each item, check it or explain why it is not applicable. -->

- [ ] New or changed kured flags are surfaced in the shipped manifests
      (`kured-ds.yaml` / `kured-ds-signal.yaml` for v1, equivalent manifests
      for v2) as comments.
- [ ] Required follow-up changes for the Helm chart are opened or noted.
- [ ] Required follow-up changes for the documentation are opened or noted.
- [ ] This pull request does not introduce user-facing flag, manifest, Helm chart, or documentation changes.

## Checklist

- [ ] I read `CONTRIBUTING.md`.
- [ ] My commits include a DCO sign-off.
- [ ] I kept the change focused and updated related tests or docs as needed.
