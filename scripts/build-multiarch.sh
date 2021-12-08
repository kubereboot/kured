#!/bin/bash

set -o nounset
set -ex

: "${VERSION:?VERISON not set}"
: "${REGISTRY:?REGISTRY not set}"
: "${WINDOWS_OS_VERSIONS:?WINDOWS_OS_VERSIONS not set}"

export DOCKER_CLI_EXPERIMENTAL=enabled
docker buildx create --name img-builder --use
trap 'docker buildx rm img-builder' EXIT
docker buildx use img-builder

# Build linux image
docker buildx build --platform linux/amd64 --output=type=registry --pull -f build/Dockerfile -t $REGISTRY/kured:$VERSION-linux ./build

# Create multi-arch image manifest
docker manifest create $REGISTRY/kured:$VERSION $REGISTRY/kured:$VERSION-linux

# Build Windows image(s) and fix os.version in container image manifest
for osversion in ${WINDOWS_OS_VERSIONS}; do \
    docker buildx build --platform windows/amd64 --output=type=registry --pull -f build/Windows.Dockerfile -t $REGISTRY/kured:$VERSION-win-$osversion --build-arg BASE_OS_VERSION=$osversion ./build
    docker manifest create --amend $REGISTRY/kured:$VERSION $REGISTRY/kured:$VERSION-win-$osversion
    full_version=$(docker manifest inspect mcr.microsoft.com/windows/nanoserver:$osversion | grep "os.version" | head -n 1 | awk -F\" '{print $4}')
    docker manifest annotate --os windows --arch amd64 --os-version $full_version $REGISTRY/kured:$VERSION $REGISTRY/kured:$VERSION-win-$osversion
done

# Print image manifest
docker manifest inspect $REGISTRY/kured:$VERSION
