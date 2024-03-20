.DEFAULT: all
.PHONY: all clean image minikube-publish manifest test kured-all

TEMPDIR=./.tmp
GORELEASER_CMD=$(TEMPDIR)/goreleaser
DH_ORG=kubereboot
VERSION=$(shell git rev-parse --short HEAD)
SUDO=$(shell docker info >/dev/null 2>&1 || echo "sudo -E")

all: image

$(TEMPDIR):
	mkdir -p $(TEMPDIR)

.PHONY: bootstrap-tools
bootstrap-tools: $(TEMPDIR)
	VERSION=v1.24.0 TMPDIR=.tmp bash .github/scripts/goreleaser-install.sh
	curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b .tmp v1.0.1
	curl -sSfL https://github.com/sigstore/cosign/releases/download/v2.2.3/cosign-linux-amd64 -o .tmp/cosign
	chmod +x .tmp/goreleaser .tmp/cosign .tmp/syft

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
	$(SUDO) docker buildx build --load -t ghcr.io/$(DH_ORG)/kured:$(VERSION) .

minikube-publish: image
	$(SUDO) docker save ghcr.io/$(DH_ORG)/kured | (eval $$(minikube docker-env) && docker load)

manifest:
	sed -i "s#image: ghcr.io/.*kured.*#image: ghcr.io/$(DH_ORG)/kured:$(VERSION)#g" kured-ds.yaml
	sed -i "s#image: ghcr.io/.*kured.*#image: ghcr.io/$(DH_ORG)/kured:$(VERSION)#g" kured-ds-signal.yaml
	echo "Please generate combined manifest if necessary"

test:
	echo "Running go tests"
	go test ./...
	echo "Running golint on pkg"
	golint ./pkg/...
	echo "Running golint on cmd"
	golint ./cmd/...
