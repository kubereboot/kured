.DEFAULT: all
.PHONY: all clean image publish-image minikube-publish

DH_ORG="quay.io/weaveworks"
VERSION=$(shell git symbolic-ref --short HEAD)-$(shell git rev-parse --short HEAD)
SUDO=$(shell docker info >/dev/null 2>&1 || echo "sudo -E")

all: image

clean:
	go clean
	rm -f cmd/kured/kured
	rm -rf ./build*

godeps=$(shell go get $1 && go list -f '{{join .Deps "\n"}}' $1 | grep -v /vendor/ | xargs go list -f '{{if not .Standard}}{{ $$dep := . }}{{range .GoFiles}}{{$$dep.Dir}}/{{.}} {{end}}{{end}}')

DEPS=$(call godeps,./cmd/kured)

cmd/kured/kured: $(DEPS)
cmd/kured/kured: cmd/kured/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o $@ cmd/kured/*.go

cmd/kured/kured-arm64: cmd/kured/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION)" -o $@ cmd/kured/*.go

build/.image.done: cmd/kured/Dockerfile cmd/kured/kured
	mkdir -p build
	cp $^ build
	$(SUDO) docker build -t $(DH_ORG)/kured -f build/Dockerfile ./build
	$(SUDO) docker tag $(DH_ORG)/kured $(DH_ORG)/kured:$(VERSION)
	touch $@

build/.image-arm64.done: cmd/kured/Dockerfile.arm64 cmd/kured/kured-arm64
	mkdir -p build-arm64
	cp $^ build-arm64
	$(SUDO) docker build -t $(DH_ORG)/kured-arm64 -f build-arm64/Dockerfile.arm64 ./build-arm64
	$(SUDO) docker tag $(DH_ORG)/kured-arm64 $(DH_ORG)/kured:$(VERSION)-arm64
	touch $@

image: build/.image.done build/.image-arm64.done

publish-image: image
	$(SUDO) docker push $(DH_ORG)/kured:$(VERSION)
	$(SUDO) docker push $(DH_ORG)/kured:$(VERSION)-arm64

minikube-publish: image
	$(SUDO) docker save $(DH_ORG)/kured | (eval $$(minikube docker-env) && docker load)
