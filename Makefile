.DEFAULT: all
.PHONY: all clean install-tools dev-image release dev-manifest e2e-test minikube-publish test lint lint-go lint-sh lint-ghactions lint-docs check-go-version

REGISTRY ?= ghcr.io
IMAGE_NAME ?= kubereboot/kured
VERSION ?= $(shell git rev-parse --short HEAD)
GORELEASER_CONFIG ?= .config/goreleaser.yaml
GOLANGCI_CONFIG ?= .config/golangci.yaml
LOCAL_PLATFORM ?= linux/amd64
SUDO=$(shell docker info >/dev/null 2>&1 || echo "sudo -E")

DEV_IMAGE := kured:dev

all: test

install-tools:
	command -v  mise 2>&1 || { echo "please install mise to continue" >&2; exit 127; }
	mise install

clean:
	rm -rf ./dist ./.tmp/goreleaser

release:
	REGISTRY="$(REGISTRY)" IMAGE_NAME="$(IMAGE_NAME)" goreleaser release --clean -f $(GORELEASER_CONFIG)

dev-image:
	mkdir -p dist/docker/$(LOCAL_PLATFORM)
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o dist/docker/$(LOCAL_PLATFORM)/kured ./cmd/kured
	cp Dockerfile dist/docker/Dockerfile
	$(SUDO) docker buildx build --load --platform $(LOCAL_PLATFORM) -t $(DEV_IMAGE) dist/docker

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

minikube-publish: dev-image
	$(SUDO) docker save $(DEV_IMAGE) | (eval $$(minikube docker-env) && docker load)

test: lint
	@echo "Running short go tests"
	go test -test.short -json ./... > test.json

lint: lint-sh lint-ghactions lint-go

check-go-version:
	@echo "Checking Go version consistency"
	@go_version="$$(sed -nE 's/^go[[:space:]]+([0-9]+\.[0-9]+\.[0-9]+)[[:space:]]*$$/\1/p' go.mod)"; \
	mise_version="$$(sed -nE 's/^go[[:space:]]*=[[:space:]]*"([0-9]+\.[0-9]+\.[0-9]+)"[[:space:]]*$$/\1/p' .config/mise.toml)"; \
	if [ -z "$$go_version" ] || [ -z "$$mise_version" ] || [ "$$go_version" != "$$mise_version" ]; then \
		echo "Go version mismatch or invalid format: go.mod=$${go_version:-invalid}, .config/mise.toml=$${mise_version:-invalid}" >&2; \
		exit 1; \
	fi

lint-sh:
	@echo "Running shellcheck"
	find . -name '*.sh' | xargs -n1 shellcheck

lint-ghactions:
	@echo "Running actionlint"
	actionlint -verbose

lint-go:
	@echo "Running golangci-lint..."
	golangci-lint run --config $(GOLANGCI_CONFIG) ./...

lint-docs:
	@echo "Running lychee"
	mise x lychee@latest -- lychee --verbose --no-progress '*.md' '*.yaml' '*/*/*.go' --exclude-link-local

lint-goreleaser:
	@echo "Checking goreleaser"
	goreleaser check

