# Developing `kured`

We love contributions to `kured`, no matter if you are [helping out on
Slack][slack], reporting or triaging [issues][issues] or contributing code
to `kured`.

In any case, it will make sense to familiarise yourself with the main
[documentation][documentation] to understand the different features and
options, which is helpful for testing. The "building" section in
particular makes sense if you are planning to contribute code.

[slack]: https://github.com/kubereboot/kured/blob/main/README.md#getting-help
[issues]: https://github.com/kubereboot/kured/issues
[documentation]: https://kured.dev/docs

## Certificate of Origin

By contributing to this project you agree to the Developer Certificate of
Origin (DCO). This document was created by the Linux Kernel community and is a
simple statement that you, as a contributor, have the legal right to make the
contribution.

We require all commits to be signed. By signing off with your signature, you
certify that you wrote the patch or otherwise have the right to contribute the
material by the rules of the [DCO](DCO):

`Signed-off-by: Jane Doe <jane.doe@example.com>`

The signature must contain your real name
(sorry, no pseudonyms or anonymous contributions)
If your `user.name` and `user.email` are configured in your Git config,
you can sign your commit automatically with `git commit -s`.

## Kured Repositories

All Kured repositories are kept under <https://github.com/kubereboot>. To find the code and work on the individual pieces that make Kured, here is our overview:

| Repositories                            | Contents                  |
| --------------------------------------- | ------------------------- |
| <https://github.com/kubereboot/kured>   | Kured operator itself     |
| <https://github.com/kubereboot/charts>  | Helm chart                |
| <https://github.com/kubereboot/website> | website and documentation |

### Kured code

- Kured's main code can be found in the [`cmd`](cmd) and [`pkg`](pkg) directories
- Its e2e tests are in the [`tests`](tests) directory
- We use [GoReleaser to build](.goreleaser.yml).
- Every PR and tagged release is tested by [Kind in GitHub workflows](.github/workflows).

As a project, we try to follow all the official and obvious standards.

## Regular development activities

### Prepare environment

Please run `make bootstrap-tools` once on a fresh repository clone to download several needed tools, e.g. GoReleaser.

### Updating k8s support

Whenever we want to update e.g. the `kubectl` or `client-go` dependencies,
some RBAC changes might be necessary too.

This is what it took to support Kubernetes 1.14:
<https://github.com/kubereboot/kured/pull/75>

That the process can be more involved based on kubernetes changes.
For example, k8s 1.10 changes to apps triggered the following commits:

b3f9ddf: Bump client-go for optimum k8s 1.10 compatibility
bc3f28d: Move deployment manifest to apps/v1
908998a: Update RBAC permissions for kubectl v1.10.3
efbb0c3: Document version compatibility in release notes
5731b98: Add warning to Dockerfile re: upgrading kubectl

Search the git log for inspiration for your cases.

Please update our `.github/workflows` with the new k8s images.

For that, run the following:

`cp .github/kind-cluster-current.yaml .github/kind-cluster-previous.yaml`
`cp .github/kind-cluster-next.yaml .github/kind-cluster-current.yaml`

Then edit `.github/kind-cluster-next.yaml` to point to the new version.

This will make the full test matrix updated (the CI and the test code).

Once your code passes all tests, update the support matrix in
the [installation docs](https://kured.dev/docs/installation/).

### Updating other dependencies

Dependabot proposes changes in our `go.mod`/`go.sum`.
Some of those changes are covered by CI testing, some are not.

Please make sure to test those not covered by CI (mostly the integration
with other tools) manually before merging.

### Review periodic jobs

We run periodic jobs (see also Automated testing section of this documentation).
Those should be monitored for failures.

If a failure happen in periodics, something terribly wrong must have happened
(or GitHub is failing at the creation of a kind cluster). Please monitor those
failures carefully.

### Introducing new features

When you introduce a new feature, the kured team expects you to have tested
your change thoroughly. If possible, include all the necessary testing in your change.

If your change involves a user facing change (change in flags of kured for example),
please include expose your new feature in our default manifest (`kured-ds.yaml`),
as a comment.

Our release manifests and helm charts are our stable interfaces.
Any user facing changes will therefore have to wait for a release before being
exposed to our users.

This also means that when you expose a new feature, you should create another PR
for your changes in <https://github.com/kubereboot/charts> to make your feature
available at the next kured version for helm users.

In the charts PR, you can directly bump the `appVersion` to the next minor version
(you are introducing a new feature, which requires a bump of the minor number.
For example, if current `appVersion` is `1.6.x`, make sure you update your `appVersion`
to `1.7.0`). It allows us to have an easy view of what we land each release.

Do not hesitate to increase the test coverage for your feature, whether it's unit
testing to full functional testing (even using helm charts).

### Increasing test coverage

We are welcoming any change to increase our test coverage.
See also our GitHub issues for the label
[`testing`](https://github.com/kubereboot/kured/labels/testing).

## Automated testing

Our CI is covered by GitHub actions.
You can see their contents in `.github/workflows`.

We currently run:

- go tests and golangci-lint
- `shellcheck`
- a check for dead links in our docs
- a security check against our base image (alpine)
- a deep functional test using our manifests on all supported k8s versions

To test your code manually, follow the section Manual testing.

## Manual (release) testing

### Quick Golang code testing

Please run `make test` to run only the basic tests. It gives a good
idea of the code behaviour.

### Linting

We use [`golangci-lint`](https://golangci-lint.run/) for Go code linting.

To run lint checks locally:

```bash
make lint
```
### Manual functional testing

Before `kured` is released, we want to make sure it still works fine on the
previous, current and next minor version of Kubernetes (with respect to the
`client-go` & `kubectl` dependencies in use). For local testing e.g.
`minikube` or `kind` can be sufficient. This will allow you to catch issues
that might not have been tested in our CI, like integration with other tools,
or your specific use case.

Deploy kured in your test scenario, make sure you pass the right `image`,
update the e.g. `period` and `reboot-days` options, so you get immediate
results, if you login to a node and run:

```console
sudo touch /var/run/reboot-required
```

### Example of functional testing with `minikube`

A test-run with `minikube` could look like this:

```cli
# start minikube
minikube start --driver=kvm2 --kubernetes-version <k8s-release>

# build kured image and publish to registry accessible by minikube
make image minikube-publish

# edit kured-ds.yaml to
#   - point to new image
#   - change e.g. period and reboot-days option for immediate results

minikube kubectl -- apply -f kured-rbac.yaml
minikube kubectl -- apply -f kured-ds.yaml
minikube kubectl -- logs daemonset.apps/kured -n kube-system -f

# In separate terminal
minikube ssh
 sudo touch /var/run/reboot-required
minikube logs -f
```

Now check for the 'Commanding reboot' message and minikube going down.

Unfortunately as of today, you are going to run into
<https://github.com/kubernetes/minikube/issues/2874>. This means that
minikube won't come back easily. You will need to start minikube again.
Then you can check for the lock release.

### Example of testing with `kind`

A test-run with `kind` could look like this:

```cli
# create kind cluster
kind create cluster --config .github/kind-cluster-<k8s-version>.yaml

# create reboot required files on pre-defined kind nodes
./tests/kind/create-reboot-sentinels.sh

# check if reboot is working fine
./tests/kind/follow-coordinated-reboot.sh

```

### Example of testing with `kind` and `make`

A test-run with `kind` and `make` can be done with the following command:

```cli
# Build kured:dev image, build manifests, and run the "long" go tests
make e2e-test
```

You can alter test behaviour by passing arguments to this command.
A few examples below:

```shell
# Run only TestE2EWithSignal test for the kubernetes version named "current" (see kind file)
make e2e-test ARGS="-run ^TestE2EWithSignal/current"
# Run all tests but make sure to extend the timeout, for slower machines.
make e2e-test ARGS="-timeout 1200s'
```

## Publishing a new kured release

### Prepare Documentation

Ensure the [compatibility matrix](https://kured.dev/docs/installation/) is
updated to the new version you want to release.

### Update the manifests with the new version

Create a commit updating the manifest with future image [like this one](https://github.com/kubereboot/kured/commit/58091f6145771f426b4b9e012a43a9c847af2560).

### Create the new version tag on the repo

Tag the previously created commit with the future release version.
The Github Actions workflow will push the new image to the registry.

### Create the combined manifest for the new version

Now create the `kured-<new version>-dockerhub.yaml` for e.g. `1.3.0`:

```sh
VERSION=1.3.0
MANIFEST="kured-$VERSION-dockerhub.yaml"
make DH_ORG="kubereboot" VERSION="${VERSION}" manifest
cat kured-rbac.yaml > "$MANIFEST"
cat kured-ds.yaml >> "$MANIFEST"
```

### Publish new version release artifacts

Now you can head to the GitHub UI for releases, drafting a new
release. Chose, as tag, the new version number.

Click to generate the release notes.

Fill, as name, "Kured <new version>".

Edit the generated text.

Please describe what's new and noteworthy in the release notes, list the PRs
that landed and give a shout-out to everyone who contributed.
Please also note down on which releases the upcoming `kured` release was
tested on or what it supports. (Check old release notes if you're unsure.)

Before clicking on publishing release, upload the yaml manifest
(`kured-<new version>-dockerhub.yaml`) file.

Click on publish the release and set as the latest release.
