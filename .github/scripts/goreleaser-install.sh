#!/bin/sh
set -e

RELEASES_URL="https://github.com/goreleaser/goreleaser/releases"
FILE_BASENAME="goreleaser"

test -z "$VERSION" && {
    echo "Unable to get goreleaser version." >&2
    exit 1
}

test -z "$TMPDIR" && TMPDIR="$(mktemp -d)"
TAR_FILE="$TMPDIR/${FILE_BASENAME}_$(uname -s)_$(uname -m).tar.gz"
export TAR_FILE

(
    echo "Downloading GoReleaser $VERSION..."
    curl -sfLo "$TAR_FILE" \
        "$RELEASES_URL/download/$VERSION/${FILE_BASENAME}_$(uname -s)_$(uname -m).tar.gz"
    cd "$TMPDIR"
    curl -sfLo "checksums.txt" "$RELEASES_URL/download/$VERSION/checksums.txt"
    echo "Verifying checksums..."
    sha256sum --ignore-missing --quiet --check checksums.txt
)

tar -xf "$TAR_FILE" -O goreleaser > "$TMPDIR/goreleaser"
rm "$TMPDIR/checksums.txt"
rm "$TAR_FILE"
