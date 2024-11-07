.DEFAULT: all
.PHONY: all clean image minikube-publish manifest test kured-all

HACKDIR=./hack/bin
GORELEASER_CMD=$(HACKDIR)/goreleaser
DH_ORG ?= kubereboot
VERSION=$(shell git rev-parse --short HEAD)
SUDO=$(shell docker info >/dev/null 2>&1 || echo "sudo -E")

all: image

$(HACKDIR):
	mkdir -p $(HACKDIR)

.PHONY: bootstrap-tools
bootstrap-tools: $(HACKDIR)
	command -v $(HACKDIR)/goreleaser || VERSION=v1.24.0 TMPDIR=$(HACKDIR) bash hack/installers/goreleaser-install.sh
	command -v  $(HACKDIR)/syft || curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b $(HACKDIR) v1.0.1
	command -v  $(HACKDIR)/cosign || curl -sSfL https://github.com/sigstore/cosign/releases/download/v2.2.3/cosign-linux-amd64 -o $(HACKDIR)/cosign
	command -v  $(HACKDIR)/shellcheck || (curl -sSfL https://github.com/koalaman/shellcheck/releases/download/stable/shellcheck-stable.linux.x86_64.tar.xz | tar -J -v -x shellcheck-stable/shellcheck && mv shellcheck-stable/shellcheck $(HACKDIR)/shellcheck && rmdir shellcheck-stable)
	chmod +x $(HACKDIR)/goreleaser $(HACKDIR)/cosign $(HACKDIR)/syft $(HACKDIR)/shellcheck
	command -v staticcheck || go install honnef.co/go/tools/cmd/staticcheck@latest

clean:
	rm -rf ./dist

kured: bootstrap-tools
	$(GORELEASER_CMD) build --clean --single-target --snapshot

kured-all: bootstrap-tools
	$(GORELEASER_CMD) build --clean --snapshot

kured-release-tag: bootstrap-tools
	$(GORELEASER_CMD) release --clean

kured-release-snapshot: bootstrap-tools
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
	# concurrency e2e command scenario
	sed -e "s#image: ghcr.io/.*kured.*#image: kured:dev#g" -e 's/#\(.*\)--period=1h/\1--period=20s/g' -e 's/#\(.*\)--concurrency=1/\1--concurrency=2/g' kured-ds.yaml > tests/kind/testfiles/kured-ds-concurrent-command.yaml
	# concurrency e2e signal scenario
	sed -e "s#image: ghcr.io/.*kured.*#image: kured:dev#g" -e 's/#\(.*\)--period=1h/\1--period=20s/g' -e 's/#\(.*\)--concurrency=1/\1--concurrency=2/g' kured-ds-signal.yaml > tests/kind/testfiles/kured-ds-concurrent-signal.yaml
	# pod blocker e2e signal scenario
	sed -e "s#image: ghcr.io/.*kured.*#image: kured:dev#g" -e 's/#\(.*\)--period=1h/\1--period=20s/g' -e 's/#\(.*\)--blocking-pod-selector=name=temperamental/\1--blocking-pod-selector=app=blocker/g' kured-ds-signal.yaml > tests/kind/testfiles/kured-ds-podblocker.yaml

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
	find . -name '*.sh' | xargs -n1 $(HACKDIR)/shellcheck
	staticcheck ./...
