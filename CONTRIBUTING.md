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

## Prepare your environment

### Your IDE

![JetBrains logo](https://resources.jetbrains.com/storage/products/company/brand/logos/jetbrains.png)

The core team has access to Goland from [JetBrains][JetBrains], thanks to their sponsorship. Huge thanks to them.

You can use the IDE you prefer. Don't include anything from your IDE in the .gitignore. Please do so in your global .gitignore.

[JetBrains]: https://www.jetbrains.com/

### Basic binaries required

Your system needs at the least the following binaries installed:

- make
- sed
- find
- bash (command, echo)
- docker (for docker buildx)
- kind
- go (see version in go.mod)

### Fetch the additional binaries required

Please run `make bootstrap-tools` once on a fresh repository clone to download several needed tools, e.g. GoReleaser.

### Configure your git for the "Certificate of Origin"

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

## Get to know Kured repositories

All Kured repositories are kept under <https://github.com/kubereboot>. To find the code and work on the individual pieces that make Kured, here is our overview:

| Repositories                            | Contents                  |
| --------------------------------------- | ------------------------- |
| <https://github.com/kubereboot/kured>   | Kured operator itself     |
| <https://github.com/kubereboot/charts>  | Helm chart                |
| <https://github.com/kubereboot/website> | website and documentation |

We use github actions in all our repositories.

### Charts repo structure highlights

- we use github actions to do the chart testing. Only linting/installation happens, no e2e test is done.
- charts/kured is the place to contribute changes to the chart. Please bump Chart.yaml at each change according to semver.

### Kured repo structure highlights

Kured's main code can be found in the [`cmd`](cmd) and [`pkg`](pkg) directories

Keep in mind we always want to guarantee that `kured` works for the previous, current, and next
minor version of kubernetes according to `client-go` and `kubectl` dependencies in use.

Our e2e tests are in the [`tests`](tests) directory. These are deep tests using our manifests with different params, on all supported k8s versions of a release.
They are expensive but allow us to catch many issues quickly. If you want to ensure your scenario works, add an e2e test for it! Those e2e tests are encouraged by the maintainer team (See below).

We also have other tests:

- golangci-lint , shellcheck
- a security check against our base image (alpine)

All these test run on every PR/tagged release. See .github/workflows for more details.

We use [GoReleaser to build](.goreleaser.yml).

## Regular development activities / maintenance

### Updating k8s support

At each new major release of kubernetes, we update our dependencies.

Beware that whenever we want to update e.g. the `kubectl` or `client-go` dependencies, some other impactful changes might be necessary too.
(RBAC, drain behaviour changes, ...)

As examples, this is what it took to support:

- Kubernetes 1.10 <https://github.com/kubereboot/kured/commit/b3f9ddf> + <https://github.com/kubereboot/kured/commit/bc3f28d> + <https://github.com/kubereboot/kured/commit/908998a> + <https://github.com/kubereboot/kured/commit/efbb0c3> + <https://github.com/kubereboot/kured/commit/5731b98>
- Kubernetes 1.14 <https://github.com/kubereboot/kured/pull/75>
- Kubernetes 1.34 <https://github.com/kubereboot/kured/commit/6ab853dd711ee264663184976ae492a20b657b0a>

Search the git log for inspiration for your cases.

In general the following activities have to happen:

- Bump kind and its images (see below)
- `go get k8s.io/kubectl@v0.{version}`

### bump kind images support

Go to `.github/workflows` and update the new k8s images. For that:

- `cp .github/kind-cluster-current.yaml .github/kind-cluster-previous.yaml`
- `cp .github/kind-cluster-next.yaml .github/kind-cluster-current.yaml`
- Then edit `.github/kind-cluster-next.yaml` to point to the new version.

This will make the full test matrix updated (the CI and the test code).

Once your code passes all tests, update the support matrix in
the [installation docs](https://kured.dev/docs/installation/).

Beware that sometimes you also need to update kind version. grep in the .github/workflows for the kind version.

### Updating other dependencies

Dependabot proposes changes in our `go.mod`/`go.sum`.
Some of those changes are covered by CI testing, some are not.

Please make sure to test those not covered by CI (mostly the integration
with other tools) manually before merging.

In all cases, review dependabot changes: Imagine all changes as a possible supply chain
attack vector. You then need to review the proposed changes by dependabot, and evaluate the trust/risks.

### Review periodic jobs

We run periodic jobs (see also Automated testing section of this documentation).
Those should be monitored for failures.

If a failure happen in periodics, something terribly wrong must have happened
(or GitHub is failing at the creation of a kind cluster). Please monitor those
failures carefully.

## Testing kured

If you have developped anything (or just want to take kured for a spin!), run the following tests.
As they will run in CI, we will quickly catch if you did not test before submitting your PR.

### Linting

We use [`golangci-lint`](https://golangci-lint.run/) for Go code linting.

To run lint checks locally:

```bash
make lint
```

### Quick Golang code testing

Please run `make test` to run only the basic tests. It gives a good
idea of the code behaviour.

### Functional testing 

For functional testing, the maintainer team is using `minikube` or `kind` (explained below), but also encourages you to test kured on your own cluster(s).

#### Testing on your own cluster

This will allow the community to catch issues that might not have been tested in our CI, like integration with other tools, or your specific use case.

To test kured on your own cluster, make sure you pass the right `image`, update the `period` and `reboot-days` (so you get immediate results), and update any other flags for your cases.
Then login to a node and run:

```console
sudo touch /var/run/reboot-required
```

Then tell us about everything that went well or went wrong in slack.

#### Testing with `minikube`

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

#### Testing with `kind` "The hard way"

A test-run with `kind` could look like this:

```cli
# create kind cluster
kind create cluster --config .github/kind-cluster-<k8s-version>.yaml

# create reboot required files on pre-defined kind nodes
./tests/kind/create-reboot-sentinels.sh

# check if reboot is working fine
./tests/kind/follow-coordinated-reboot.sh

```

### Testing with `kind` "The easy way"

You can automate the test with `kind` by using the same code as the CI.

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

## Introducing new features

When you introduce a new feature, the kured team expects you to have tested (see above!)
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

The team of kured is small, so we will most likely refuse any feature adding maintenance burden.

## Introducing changes in the helm chart

When you change the helm chart, do not forget to bump its version according to semver.
Changes to defaults are frowned upon unless absolutely necessary.

## Introducing new tests / increase test coverage

At the opposite of features, we welcome ALL features increasing our stability and test coverage.
See also our GitHub issues with the label [`testing`](https://github.com/kubereboot/kured/labels/testing).

## Publishing a new kured release

### Double check the latest kubernetes patch version

Ensure you have used the latest patch version in tree. 
Check the documentation "Updating k8s support" if the minor version was not yet applied.

### Update the manifests with the new version

```sh
export VERSION=1.20.0
make DH_ORG="kubereboot" VERSION="${VERSION}" manifest
```
Create a commit updating the manifest with future image [like this one](https://github.com/kubereboot/kured/commit/58091f6145771f426b4b9e012a43a9c847af2560).

### Create the combined manifest for the new release

Now create the `kured-<new version>-combined.yaml` for e.g. `1.20.0`:

```sh
export VERSION=1.20.0
export MANIFEST="kured-$VERSION-combined.yaml"
make DH_ORG="kubereboot" VERSION="${VERSION}" manifest # just to be safe
cat kured-rbac.yaml > "$MANIFEST"
cat kured-ds.yaml >> "$MANIFEST"
```

### Create the new version tag on the repo (optional, can also be done directly in GH web interface)

Tag the previously created commit with the future release version.
The Github Actions workflow will push the new image to the registry.

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
(`kured-<new version>-combined.yaml`) file.

Click on publish the release and set as the latest release.

### Prepare Helm chart

Create a commit to [bump the chart and kured version like this one](https://github.com/kubereboot/charts/commit/e0191d91c21db8338be8cbe56f8991a557048110).

### Prepare Documentation

Ensure the [compatibility matrix](https://kured.dev/docs/installation/) is updated to the new version you want to release.

