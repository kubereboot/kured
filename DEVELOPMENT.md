# Developing `kured`

We love contributions to `kured`, no matter if you are [helping out on
Slack][slack], reporting or triaging [issues][issues] or contributing code
to `kured`.

In any case, it will make sense to familiarise yourself with the main
[README][readme] to understand the different features and options, which is
helpful for testing. The "building" section in particular makes sense if
you are planning to contribute code.

[slack]: README.md#getting-help
[issues]: https://github.com/weaveworks/kured/issues
[readme]: README.md

## Regular development activities

### Updating k8s support

Whenever we want to update e.g. the `kubectl` or `client-go` dependencies,
some RBAC changes might be necessary too.

This is what it took to support Kubernetes 1.14:
<https://github.com/weaveworks/kured/pull/75>

That the process can be more involved that that can be seen in
<https://github.com/weaveworks/kured/commits/support-k8s-1.10>

Please update our .github/workflows with the new k8s images, starting by
the creation of a .github/kind-cluster-<version>.yaml, then updating
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

Do not update the helm chart directly.
Helm charts and our release manifests (see below) are our stable interfaces.
Any user facing changes will therefore have to wait for a while before being
exposed to our users.

This also means that when you expose a new feature, you should create another PR
for your changes in `charts/` to make your feature available for our next kured version.
In this change, you can directly bump the appVersion to the next minor version.
(for example, if current appVersion is 1.6.x, make sure you update your appVersion
to 1.7.0). It allows us to have an easy view of what we land each release.

Do not hesitate to increase the test coverage for your feature, whether it's unit
testing to full functional testing (even using helm charts)

### Increasing test coverage

We are welcoming any change to increase our test coverage.
See also our github issues for the label `testing`.

### Updating helm charts

Helm charts are continuously published. Any change in `charts/` will be immediately
pushed in production.

## Automated testing

Our CI is covered by github actions.
You can see their contents in .github/workflows.

We currently run:
- go tests and lint
- shellcheck
- a check for dead links in our docs
- a security check against our base image (alpine)
- a deep functional test using our manifests on all supported k8s versions
- basic deployment using our helm chart on any chart change

Changes in helm charts are not functionally tested on PRs. We assume that
the PRs to implement the feature are properly tested by our users and
contributors before merge.

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

Please run `make test`. You should have golint installed.

### Example of testing with `minikube`

A test-run with `minikube` could look like this:

```console
# start minikube
minikube start --vm-driver kvm2 --kubernetes-version <k8s-release>

# build kured image and publish to registry accessible by minikube
make image minikube-publish

# edit kured-ds.yaml to
#   - point to new image
#   - change e.g. period and reboot-days option for immediate results

minikube kubectl -- apply -f kured-rbac.yaml
minikube kubectl -- apply -f kured-ds.yaml
minikube kubectl -- logs daemonset.apps/kured -n kube-system -f

# Alternatively use helm to install the chart
# edit values-local.yaml to change any chart parameters
helm install kured ./charts/kured --namespace kube-system -f ./charts/kured/values.minikube.yaml

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

If all the tests ran well, kured maintainers can reach out to the Weaveworks
team to get an upcoming `kured` release tested in the Dev environment for
real life testing.

### Example of testing with `kind`

A test-run with `kind` could look like this:

```console
# create kind cluster
kind create cluster --config .github/kind-cluster-<k8s-version>.yaml

# load the image to the kind cluster
kind load docker-image docker.io/weaveworks/kured

# create reboot required files on pre-defined kind nodes
./tests/kind/create-reboot-sentinels.sh

# check if reboot is working fine
./tests/kind/follow-coordinated-reboot.sh

```

## Publishing a new kured release

### Prepare Documentation

Check that `README.md` has an updated compatibility matrix and that the
url in the `kubectl` incantation (under "Installation") is updated to the
new version you want to release.

### Create a tag on the repo and publish the image

Before going further, we should freeze the code for a release, by
tagging the code, and publishing its immutable artifact: the kured
docker image.

```sh
make DH_ORG="weaveworks" VERSION="1.3.0" image
```

Then docker push the image. In the future, that might be automatically
done when creating a tag on the repository, with the help of github
actions.

### Create the combined manifest

Now create the `kured-<release>-dockerhub.yaml` for e.g. `1.3.0`:

```sh
VERSION=1.3.0
MANIFEST="kured-$VERSION-dockerhub.yaml"
make DH_ORG="weaveworks" VERSION="${VERSION}" manifest
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

### Update the Helm chart

You can automatically bump the helm chart's application version
with the latest image tag by running:

```sh
make DH_ORG="weaveworks" VERSION="1.3.0" helm-chart
```

A change in the helm chart requires a bump of the `version`
in `charts/kured/Chart.yaml` (following the versioning rules).
Update it, and issue a PR. Upon merge, that PR will automatically
publish the chart to the gh-pages branch.
