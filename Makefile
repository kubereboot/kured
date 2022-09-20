.DEFAULT: all
.PHONY: all clean image publish-image minikube-publish manifest test tests kured-multi

DH_ORG=kubereboot
VERSION=$(shell git symbolic-ref --short HEAD)-$(shell git rev-parse --short HEAD)
SUDO=$(shell docker info >/dev/null 2>&1 || echo "sudo -E")

all: image

clean:
	rm -f cmd/kured/kured
	rm -rf ./build

godeps=$(shell go list -f '{{join .Deps "\n"}}' $1 | grep -v /vendor/ | xargs go list -f '{{if not .Standard}}{{ $$dep := . }}{{range .GoFiles}}{{$$dep.Dir}}/{{.}} {{end}}{{end}}')

DEPS=$(call godeps,./cmd/kured)

cmd/kured/kured: $(DEPS)
cmd/kured/kured: cmd/kured/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o $@ cmd/kured/*.go

kured-multi: 
	CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o cmd/kured/kured cmd/kured/*.go

build/.image.done: cmd/kured/Dockerfile cmd/kured/kured
	mkdir -p build
	cp $^ build
	$(SUDO) docker build -t ghcr.io/$(DH_ORG)/kured -f build/Dockerfile ./build
	$(SUDO) docker tag ghcr.io/$(DH_ORG)/kured ghcr.io/$(DH_ORG)/kured:$(VERSION)
	touch $@

image: build/.image.done

publish-image: image
	$(SUDO) docker push ghcr.io/$(DH_ORG)/kured:$(VERSION)

minikube-publish: image
	$(SUDO) docker save ghcr.io/$(DH_ORG)/kured | (eval $$(minikube docker-env) && docker load)

manifest:
	sed -i "s#image: docker.io/.*kured.*#image: docker.io/$(DH_ORG)/kured:$(VERSION)#g" kured-ds.yaml
	echo "Please generate combined manifest if necessary"

test: tests
	echo "Running go tests"
	go test ./...
	echo "Running golint on pkg"
	golint ./pkg/...
	echo "Running golint on cmd"
	golint ./cmd/...
