# Developing `kured`

We love contributions to `kured`, no matter if you are [helping out on
Slack][slack], reporting or triaging [issues][issues] or contributing code
to `kured`.

In any case, it will make sense to familiarise yourself with the main
[README][readme] to understand the different features and options, which is
helpful for testing. The "building" section in particular makes sense if
you are planning to contribute code.

[slack]: README.md#getting-help
[issues]: https://github.com/kubereboot/kured/issues
[readme]: README.md

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

Please update our `.github/workflows` with the new k8s images, starting by
the creation of a `.github/kind-cluster-<version>.yaml`, then updating
our workflows with the new versions.

Once you updated everything, make sure you update the support matrix on
the main [README][readme] as well.

### Updating other dependencies

Dependabot proposes changes in our go.mod/go.sum.
Some of those changes are covered by CI testing, some are not.

Please make sure to test those not covered by CI (mostly the integration
with other tools) manually before merging.

### Review periodic jobs

We run periodic jobs (see also Automated testing section of this documentation).
Those should be monitored for failures.

If a failure happen in periodics, something terribly wrong must have happened
(or github is failing at the creation of a kind cluster). Please monitor those
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

In the charts PR, you can directly bump the appVersion to the next minor version
(you are introducing a new feature, which requires a bump of the minor number.
For example, if current appVersion is 1.6.x, make sure you update your appVersion
to 1.7.0). It allows us to have an easy view of what we land each release.

Do not hesitate to increase the test coverage for your feature, whether it's unit
testing to full functional testing (even using helm charts)

### Increasing test coverage

We are welcoming any change to increase our test coverage.
See also our github issues for the label `testing`.

## Automated testing

Our CI is covered by github actions.
You can see their contents in .github/workflows.

We currently run:

- go tests and lint
- `shellcheck`
- a check for dead links in our docs
- a security check against our base image (alpine)
- a deep functional test using our manifests on all supported k8s versions

To test your code manually, follow the section Manual testing.

## Manual (release) testing

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

### Example of golang testing

Please run `make test`. You should have `golint` installed.

### Example of testing with `minikube`

A test-run with `minikube` could look like this:

```console
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

```console
# create kind cluster
kind create cluster --config .github/kind-cluster-<k8s-version>.yaml

# create reboot required files on pre-defined kind nodes
./tests/kind/create-reboot-sentinels.sh

# check if reboot is working fine
./tests/kind/follow-coordinated-reboot.sh

```

## Publishing a new kured release

### Prepare Documentation

Check that [compatibility matrix](https://kured.dev/docs/installation/) is updated
to the new version you want to release.

### Create a tag on the repo

Before going further, we should freeze the code for a release, by
tagging the code. The Github-Action should start a new job and push
the new image to the registry.

### Create the combined manifest

Now create the `kured-<release>-dockerhub.yaml` for e.g. `1.3.0`:

```sh
VERSION=1.3.0
MANIFEST="kured-$VERSION-dockerhub.yaml"
make DH_ORG="kubereboot" VERSION="${VERSION}" manifest
cat kured-rbac.yaml > "$MANIFEST"
cat kured-ds.yaml >> "$MANIFEST"
```

### Publish release artifacts

Now you can head to the Github UI, use the version number as tag and upload the
`kured-<release>-dockerhub.yaml` file.

Please describe what's new and noteworthy in the release notes, list the PRs
that landed and give a shout-out to everyone who contributed.

Please also note down on which releases the upcoming `kured` release was
tested on. (Check old release notes if you're unsure.)
