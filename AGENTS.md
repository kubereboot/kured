# Agent Guidance

Use the `Makefile` as the source of truth for development commands. Read
`CONTRIBUTING.md` for broader contribution, DCO, and release requirements.

## Repository Layout

- `cmd/kured` contains the CLI, configuration loading, and main controller.
- `pkg` contains the exportable and independently testable components.
   It should cover the checking sentinels/blocking reboot/rebooting/notifying of events.
   In case you want a new component to expose, ask humans for guidance by
   contacting the maintainers of this project.
- `internal` contains go code to not export. This is where most of the code will
   reside.
- `tests/kind` contains the Kubernetes end-to-end scenarios.

## Development and Verification

- Run `make install-tools` on a fresh checkout to install the tools pinned in
  `.config/mise.toml`.
- Format Go changes with `gofmt` and add tests beside the changed package.
- Use targeted short tests while developing, for example
  `go test -short ./pkg/timewindow` or `go test -short ./pkg/blockers -run TestActiveAlerts`.
- Run `make test` for the standard lint and short-test suite.
- Run `make lint-docs` for Markdown or manifest documentation changes and
  `make lint-goreleaser` for GoReleaser configuration changes.
- Every contribution must have documentation.
- Every contribution must pass the complete `make e2e-test` suite before it is
  considered complete. A targeted `ARGS` run is useful during development but
  does not replace the final full run. Report any failure or inability to run
  the suite; do not silently omit it.

The Kind e2e tests are expensive, create clusters, and run scenarios in
parallel. Existing e2e scenarios and assertions must not be removed, skipped,
weakened, or replaced. Changes to e2e coverage may only add new scenarios.

When running kind e2e test, ask for parallelization and duration level to the
human trigger.

## Project Invariants

## Go Version Updates

The Go version is pinned in both `go.mod` and `.config/mise.toml`. When bumping
Go, update both files to the same full patch version and verify that no old
version references remain in the repository.

## Kubernetes and User-Facing Changes

- Keep the `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`, and
  `k8s.io/kubectl` module versions aligned. Treat these updates as compatibility
  changes and review Kind configurations, drain behavior, and `kured-rbac.yaml`.
- For user-facing flag changes, update and test flag parsing and environment
  loading, expose the option in `kured-ds.yaml`, and assess whether
  `kured-ds-signal.yaml` needs the equivalent change.
- Release manifests are stable interfaces. Do not silently change defaults.

## Generated Files

Do not hand-edit `dist` or the ignored YAML files under
`tests/kind/testfiles`. Regenerate them with `make dev-image` or
`make dev-manifest` as appropriate.
