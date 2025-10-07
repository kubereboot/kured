.DEFAULT: all
.PHONY: all clean image minikube-publish manifest test kured-all lint 

DH_ORG ?= kubereboot
VERSION=$(shell git rev-parse --short HEAD)
SUDO=$(shell docker info >/dev/null 2>&1 || echo "sudo -E")

all: image

.PHONY: install-tools
install-tools:
	command -v  mise 2>&1 || { echo "please install mise to continue" >&2; exit 127; }
	mise install

clean:
	rm -rf ./dist

kured:
	goreleaser build --clean --single-target --snapshot

kured-all:
	goreleaser build --clean --snapshot

kured-release-tag:
	goreleaser release --clean

kured-release-snapshot:
	goreleaser release --clean --snapshot

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

test: lint
	@echo "Running short go tests"
	go test -test.short -json ./... > test.json

lint:
	@echo "Running shellcheck"
	find . -name '*.sh' | xargs -n1 shellcheck
	@echo "Running golangci-lint..."
	golangci-lint run ./...

lint-docs:
	@echo "Running lychee"
	mise x lychee@latest -- lychee --verbose --no-progress '*.md' '*.yaml' '*/*/*.go' --exclude-link-local