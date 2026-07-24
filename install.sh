#!/bin/sh
# speclib installer.
#
#   curl -fsSL https://raw.githubusercontent.com/jacob-meacham/speclib/main/install.sh | sh
#
# Environment overrides:
#   SPECLIB_VERSION      version to install, with or without the leading "v"
#                        (default: the latest release)
#   SPECLIB_INSTALL_DIR  directory to install into (default: /usr/local/bin)
set -eu

REPO="jacob-meacham/speclib"
INSTALL_DIR="${SPECLIB_INSTALL_DIR:-/usr/local/bin}"

fail() {
    echo "install.sh: $1" >&2
    exit 1
}

command -v curl >/dev/null 2>&1 || fail "curl is required"

case "$(uname -s)" in
    Linux) os="linux" ;;
    Darwin) os="darwin" ;;
    *) fail "unsupported OS $(uname -s): binaries are published for linux and darwin only. Try: go install github.com/$REPO@latest" ;;
esac

case "$(uname -m)" in
    x86_64 | amd64) arch="amd64" ;;
    aarch64 | arm64) arch="arm64" ;;
    *) fail "unsupported architecture $(uname -m): binaries are published for amd64 and arm64 only. Try: go install github.com/$REPO@latest" ;;
esac

version="${SPECLIB_VERSION:-}"
if [ -z "$version" ]; then
    # Follow the releases/latest redirect to .../tag/vX.Y.Z. Unlike the GitHub
    # API, the redirect is not rate-limited for unauthenticated callers.
    latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest") \
        || fail "could not resolve the latest release of $REPO"
    version="${latest_url##*/}"
    [ "$version" != "latest" ] && [ -n "$version" ] \
        || fail "could not parse a release tag from $latest_url (no releases yet?)"
fi
version="${version#v}"

tarball="speclib_${version}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/v$version"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading speclib v$version ($os/$arch)..."
curl -fsSL -o "$tmp/$tarball" "$base/$tarball" \
    || fail "download failed: $base/$tarball (does v$version publish $os/$arch?)"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" \
    || fail "download failed: $base/checksums.txt"

verify_checksum() {
    if command -v sha256sum >/dev/null 2>&1; then
        grep " $1\$" checksums.txt | sha256sum -c - >/dev/null
    elif command -v shasum >/dev/null 2>&1; then
        grep " $1\$" checksums.txt | shasum -a 256 -c - >/dev/null
    else
        fail "need sha256sum or shasum to verify the download"
    fi
}
(cd "$tmp" && verify_checksum "$tarball") \
    || fail "checksum verification failed for $tarball"

tar -xzf "$tmp/$tarball" -C "$tmp" speclib

if [ ! -d "$INSTALL_DIR" ]; then
    mkdir -p "$INSTALL_DIR" 2>/dev/null \
        || sudo mkdir -p "$INSTALL_DIR" \
        || fail "could not create $INSTALL_DIR"
fi
if [ -w "$INSTALL_DIR" ]; then
    install -m 0755 "$tmp/speclib" "$INSTALL_DIR/speclib"
else
    echo "$INSTALL_DIR is not writable; escalating to sudo for the copy..."
    sudo install -m 0755 "$tmp/speclib" "$INSTALL_DIR/speclib" \
        || fail "could not install to $INSTALL_DIR (set SPECLIB_INSTALL_DIR to a writable directory)"
fi

case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) echo "note: $INSTALL_DIR is not on your PATH" >&2 ;;
esac

echo "Installed $("$INSTALL_DIR/speclib" --version) to $INSTALL_DIR/speclib"
