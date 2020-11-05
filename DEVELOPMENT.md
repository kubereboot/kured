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

## Updating k8s support

Whenever we want to update e.g. the `kubectl` or `client-go` dependencies,
some RBAC changes might be necessary too.

This is what it took to support Kubernetes 1.14:
<https://github.com/weaveworks/kured/pull/75>

That the process can be more involved that that can be seen in
<https://github.com/weaveworks/kured/commits/support-k8s-1.10>

Once you updated everything, make sure you update the support matrix on
the main [README][readme] as well.

## Release testing

Before `kured` is released, we want to make sure it still works fine on the
previous, current and next minor version of Kubernetes (with respect to the
embedded `client-go` & `kubectl`). For local testing e.g. `minikube` can be
sufficient.

Deploy kured in your test scenario, make sure you pass the right `image`,
update the e.g. `period` and `reboot-days` options, so you get immediate
results, if you login to a node and run:

```console
sudo touch /var/run/reboot-required
```

### Testing with `minikube`

A test-run with `minikube` could look like this:

```console
minikube start --vm-driver kvm2 --kubernetes-version <k8s-release>

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

If all the tests ran well, kured maintainers can reach out to the Weaveworks
team to get an upcoming `kured` release tested in the Dev environment for
real life testing.

## Publishing a new kured release

Check that `README.md` has an updated compatibility matrix and that the
url in the `kubectl` incantation (under "Installation") is updated to the
new version you want to release.

Now create the `kured-<release>-dockerhub.yaml` for e.g. `1.3.0`:

```sh
VERSION=1.3.0
MANIFEST="kured-$VERSION-dockerhub.yaml"
cat kured-rbac.yaml > "$MANIFEST"
cat kured-ds.yaml >> "$MANIFEST"
sed -i "s#docker.io/weaveworks/kured#docker.io/weaveworks/kured:$VERSION#g" "$MANIFEST"
```

To make this available for our Helm users, please make sure you update the
image version in

- `charts/kured/values.yaml` (`tag`),
- `charts/kured/Chart.yaml` (`appVersion`) and
- `charts/kured/README.md` (`image.tag`) as well.

Finally bump the `version` in `charts/kured/Chart.yaml` and you should be
all set.

Now you can head to the Github UI, use the version number as tag and upload the
`kured-<release>-dockerhub.yaml` file.

### Release notes

Please describe what's new and noteworthy in the release notes, list the PRs
that landed and give a shout-out to everyone who contributed.

Please also note down on which releases the upcoming `kured` release was
tested on. (Check old release notes if you're unsure.)
