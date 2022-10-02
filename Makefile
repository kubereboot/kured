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
	VERSION=v1.11.4 TMPDIR=.tmp bash .github/scripts/goreleaser-install.sh
	curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b .tmp v0.58.0
	curl -sSfL https://github.com/sigstore/cosign/releases/download/v1.12.1/cosign-linux-amd64 -o .tmp/cosign
	chmod +x .tmp/goreleaser .tmp/cosign .tmp/syft

clean:
	rm -rf ./dist

kured:
	$(GORELEASER_CMD) build --rm-dist --single-target --snapshot

kured-all:
	$(GORELEASER_CMD) build --rm-dist --snapshot

kured-release-tag:
	$(GORELEASER_CMD) release --rm-dist

kured-release-snapshot:
	$(GORELEASER_CMD) release --rm-dist --snapshot

image: kured
	$(SUDO) docker buildx build --load -t ghcr.io/$(DH_ORG)/kured:$(VERSION) .

minikube-publish: image
	$(SUDO) docker save ghcr.io/$(DH_ORG)/kured | (eval $$(minikube docker-env) && docker load)

manifest:
	sed -i "s#image: ghcr.io/.*kured.*#image: ghcr.io/$(DH_ORG)/kured:$(VERSION)#g" kured-ds.yaml
	echo "Please generate combined manifest if necessary"

test:
	echo "Running go tests"
	go test ./...
	echo "Running golint on pkg"
	golint ./pkg/...
	echo "Running golint on cmd"
	golint ./cmd/...
