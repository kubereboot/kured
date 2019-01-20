.PHONY: all clean all-publish build-image publish-image publish-manifest minikube-publish

DH_ORG:=weaveworks
DH_IMAGE:=$(DH_ORG)/kured
VERSION:=$(shell git symbolic-ref --short HEAD)-$(shell git rev-parse --short HEAD)
SUDO:=$(shell docker info >/dev/null 2>&1 || echo "sudo -E")
ALL_ARCHES:=amd64 arm arm64

# This option is for running the docker manifest command
export DOCKER_CLI_EXPERIMENTAL := enabled

ifeq ($(ARCH),amd64)
	BASEIMAGE?=alpine:3.8
endif
ifeq ($(ARCH),arm)
	BASEIMAGE?=arm32v6/alpine:3.8
endif
ifeq ($(ARCH),arm64)
	BASEIMAGE?=arm64v8/alpine:3.8
endif

all: all-build

all-build: $(addprefix build/kured-,$(ALL_ARCHES))

sub-publish-image-%:
	$(MAKE) ARCH=$* publish-image

all-publish-image: $(addprefix sub-publish-image-,$(ALL_ARCHES))

all-publish: all-build all-publish-image publish-manifest

clean:
	go clean
	rm -f cmd/kured/kured*
	rm -rf ./build/

godeps=$(shell go get $1 && go list -f '{{join .Deps "\n"}}' $1 | grep -v /vendor/ | xargs go list -f '{{if not .Standard}}{{ $$dep := . }}{{range .GoFiles}}{{$$dep.Dir}}/{{.}} {{end}}{{end}}')

DEPS=$(call godeps,./cmd/kured)

cmd/kured/kured: $(DEPS)
build/kured-%: cmd/kured/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=$* go build -ldflags "-X main.version=$(VERSION)" -o $@ $^
	$(MAKE) ARCH=$* build-image

build-image:
	cat Dockerfile.build | sed "s|BASEIMAGE|$(BASEIMAGE)|g" | sed "s|ARCH|$(ARCH)|g" \
		> build/Dockerfile-$(ARCH)
	$(SUDO) docker build -t $(DH_IMAGE) -f build/Dockerfile-$(ARCH) ./build
	@# We need to specify arch here for manifest build/push to work correctly
	$(SUDO) docker tag $(DH_IMAGE) $(DH_ORG)/kured:$(VERSION)-$(ARCH)

publish-image:
	$(SUDO) docker push $(DH_IMAGE):$(VERSION)-$(ARCH)

publish-manifest:
	docker manifest create --amend $(DH_IMAGE):$(VERSION) $(shell echo $(ALL_ARCHES) | sed -e "s~[^ ]*~$(DH_IMAGE):$(VERSION)\-&~g")
	@for arch in $(ALL_ARCHES); do \
		docker manifest annotate --arch $${arch} ${DH_IMAGE}:${VERSION} ${DH_IMAGE}:${VERSION}-$${arch}; \
	done
	docker manifest push --purge ${DH_IMAGE}:${VERSION}

minikube-publish:
	$(SUDO) docker save $(DH_ORG)/kured | (eval $$(minikube docker-env) && docker load)
