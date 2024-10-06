.DEFAULT: all
.PHONY: all clean image minikube-publish manifest test kured-all

TEMPDIR=./.tmp
GORELEASER_CMD=$(TEMPDIR)/goreleaser
DH_ORG ?= kubereboot
VERSION=$(shell git rev-parse --short HEAD)
SUDO=$(shell docker info >/dev/null 2>&1 || echo "sudo -E")

all: image

$(TEMPDIR):
	mkdir -p $(TEMPDIR)

.PHONY: bootstrap-tools
bootstrap-tools: $(TEMPDIR)
	command -v .tmp/goreleaser || VERSION=v1.24.0 TMPDIR=.tmp bash .github/scripts/goreleaser-install.sh
	command -v  .tmp/syft || curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b .tmp v1.0.1
	command -v  .tmp/cosign || curl -sSfL https://github.com/sigstore/cosign/releases/download/v2.2.3/cosign-linux-amd64 -o .tmp/cosign
	command -v  .tmp/shellcheck || (curl -sSfL https://github.com/koalaman/shellcheck/releases/download/stable/shellcheck-stable.linux.x86_64.tar.xz | tar -J -v -x shellcheck-stable/shellcheck && mv shellcheck-stable/shellcheck .tmp/shellcheck && rmdir shellcheck-stable)
	chmod +x .tmp/goreleaser .tmp/cosign .tmp/syft .tmp/shellcheck
	# go install honnef.co/go/tools/cmd/staticcheck@latest

clean:
	rm -rf ./dist

kured:
	$(GORELEASER_CMD) build --clean --single-target --snapshot

kured-all:
	$(GORELEASER_CMD) build --clean --snapshot

kured-release-tag:
	$(GORELEASER_CMD) release --clean

kured-release-snapshot:
	$(GORELEASER_CMD) release --clean --snapshot

image: kured
	$(SUDO) docker buildx build --no-cache --load -t ghcr.io/$(DH_ORG)/kured:$(VERSION) .

dev-image: image
	$(SUDO) docker tag ghcr.io/$(DH_ORG)/kured:$(VERSION) kured:dev

dev-manifest:
	# basic e2e scenario
	sed -e "s#image: ghcr.io/.*kured.*#image: kured:dev#g" -e 's/#\(.*\)--period=1h/\1--period=20s/g' kured-ds.yaml > tests/kind/testfiles/kured-ds.yaml
	# signal e2e scenario
	sed -e "s#image: ghcr.io/.*kured.*#image: kured:dev#g" -e 's/#\(.*\)--period=1h/\1--period=20s/g' kured-ds-signal.yaml > tests/kind/testfiles/kured-ds-signal.yaml
	# concurrency e2e scenario
	sed -e "s#image: ghcr.io/.*kured.*#image: kured:dev#g" -e 's/#\(.*\)--period=1h/\1--period=20s/g' -e 's/#\(.*\)--concurrency=1/\1--concurrency=2/g' kured-ds.yaml > tests/kind/testfiles/kured-ds-concurrent.yaml


e2e-test: dev-manifest dev-image
	echo "Running ALL go tests"
	go test -count=1 -v --parallel 4 ./... $(ARGS)

minikube-publish: image
	$(SUDO) docker save ghcr.io/$(DH_ORG)/kured | (eval $$(minikube docker-env) && docker load)

manifest:
	sed -i "s#image: ghcr.io/.*kured.*#image: ghcr.io/$(DH_ORG)/kured:$(VERSION)#g" kured-ds.yaml
	sed -i "s#image: ghcr.io/.*kured.*#image: ghcr.io/$(DH_ORG)/kured:$(VERSION)#g" kured-ds-signal.yaml
	echo "Please generate combined manifest if necessary"

test: bootstrap-tools
	echo "Running short go tests"
	go test -test.short -json ./... > test.json
	echo "Running shellcheck"
	find . -name '*.sh' -exec .tmp/shellcheck  {} \;
	# Need to add staticcheck to replace golint as golint is deprecated, and staticcheck is the recommendation
