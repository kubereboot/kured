.DEFAULT: all
.PHONY: all clean image publish-image minikube-publish manifest helm-chart test tests

DH_ORG=weaveworks
VERSION=$(shell git symbolic-ref --short HEAD)-$(shell git rev-parse --short HEAD)
SUDO=$(shell docker info >/dev/null 2>&1 || echo "sudo -E")
WINDOWS_OS_VERSIONS="1809 ltsc2022"

all: image

clean:
	rm -f cmd/kured/kured
	rm -rf ./build

godeps=$(shell go list -f '{{join .Deps "\n"}}' $1 | grep -v /vendor/ | xargs go list -f '{{if not .Standard}}{{ $$dep := . }}{{range .GoFiles}}{{$$dep.Dir}}/{{.}} {{end}}{{end}}')

DEPS=$(call godeps,./cmd/kured)

cmd/kured/kured: $(DEPS)
cmd/kured/kured: cmd/kured/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o $@ cmd/kured/*.go
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o $@.exe cmd/kured/*.go

build/.image.done: cmd/kured/*Dockerfile cmd/kured/kured* scripts/*
	mkdir -p build
	cp $^ build
	REGISTRY=docker.io/$(DH_ORG) VERSION=$(VERSION) WINDOWS_OS_VERSIONS=$(WINDOWS_OS_VERSIONS) $(SUDO) ./build/build-multiarch.sh
	REGISTRY=ghcr.io/$(DH_ORG) VERSION=$(VERSION) WINDOWS_OS_VERSIONS=$(WINDOWS_OS_VERSIONS) $(SUDO) ./build/build-multiarch.sh
	touch $@

image: build/.image.done

publish-image: image
	$(SUDO) docker manifest push docker.io/$(DH_ORG)/kured:$(VERSION)
	$(SUDO) docker manifest push ghcr.io/$(DH_ORG)/kured:$(VERSION)

minikube-publish: image
	$(SUDO) docker image pull docker.io/$(DH_ORG)/kured:$(VERSION)-linux
	$(SUDO) docker image tag docker.io/$(DH_ORG)/kured:$(VERSION)-linux docker.io/$(DH_ORG)/kured
	$(SUDO) docker save docker.io/$(DH_ORG)/kured | (eval $$(minikube docker-env) && docker load)

manifest:
	sed -i "s#image: docker.io/.*kured.*#image: docker.io/$(DH_ORG)/kured:$(VERSION)#g" kured-ds.yaml
	echo "Please generate combined manifest if necessary"

helm-chart:
	sed -i "s#repository:.*/kured#repository: $(DH_ORG)/kured#g" charts/kured/values.yaml
	sed -i "s#appVersion:.*#appVersion: \"$(VERSION)\"#g" charts/kured/Chart.yaml
	sed -i "s#\`[0-9]*\.[0-9]*\.[0-9]*\`#\`$(VERSION)\`#g" charts/kured/README.md
	echo "Please bump version in charts/kured/Chart.yaml"

test: tests
	echo "Running go tests"
	go test ./...
	echo "Running golint on pkg"
	golint ./pkg/...
	echo "Running golint on cmd"
	golint ./cmd/...
