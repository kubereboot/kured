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
export $TAR_FILE

(
    echo "Downloading GoReleaser $VERSION..."
    curl -sfLo "$TAR_FILE" \
        "$RELEASES_URL/download/$VERSION/${FILE_BASENAME}_$(uname -s)_$(uname -m).tar.gz"
    cd "$TMPDIR"
    curl -sfLo "checksums.txt" "$RELEASES_URL/download/$VERSION/checksums.txt"
    curl -sfLo "checksums.txt.sig" "$RELEASES_URL/download/$VERSION/checksums.txt.sig"
    echo "Verifying checksums..."
    sha256sum --ignore-missing --quiet --check checksums.txt
    if command -v cosign >/dev/null 2>&1; then
        echo "Verifying signatures..."
        COSIGN_EXPERIMENTAL=1 cosign verify-blob \
            --signature checksums.txt.sig \
            checksums.txt
    else
        echo "Could not verify signatures, cosign is not installed."
    fi
)

tar -xf "$TAR_FILE" -O goreleaser > "$TMPDIR/goreleaser"
rm "$TMPDIR/checksums.txt" "$TMPDIR/checksums.txt.sig"
rm "$TAR_FILE"
