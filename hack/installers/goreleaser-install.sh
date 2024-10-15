#!/bin/sh
set -e

RELEASES_URL="https://github.com/goreleaser/goreleaser/releases"
FILE_BASENAME="goreleaser"

test -z "$VERSION" && {
    echo "Unable to get goreleaser version." >&2
    exit 1
}

test -z "$TMPDIR" && TMPDIR="$(mktemp -d)"
# goreleaser uses arm64 instead of aarch64
goreleaser_arch=$(uname -m | sed -e 's/aarch64/arm64/g' -e 's/ppc64le/ppc64/' -e 's/armv7l/armv7/' )
TAR_FILE="$TMPDIR/${FILE_BASENAME}_$(uname -s)_${goreleaser_arch}.tar.gz"
export TAR_FILE

(
    echo "Downloading GoReleaser $VERSION..."
    curl -sfLo "$TAR_FILE" \
        "$RELEASES_URL/download/$VERSION/${FILE_BASENAME}_$(uname -s)_${goreleaser_arch}.tar.gz"
    cd "$TMPDIR"
    curl -sfLo "checksums.txt" "$RELEASES_URL/download/$VERSION/checksums.txt"
    echo "Verifying checksums..."
    sha256sum --ignore-missing --quiet --check checksums.txt
)

tar -xf "$TAR_FILE" -O goreleaser > "$TMPDIR/goreleaser"
rm "$TMPDIR/checksums.txt"
rm "$TAR_FILE"
