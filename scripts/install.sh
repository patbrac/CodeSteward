#!/bin/sh
# CodeSteward installer.
#
# Downloads a release binary from GitHub, optionally verifies its checksum
# against SHA256SUMS, and installs it.
#
# Environment variables:
#   VERSION                  release tag to install (e.g. v0.1.0); default latest
#   CODESTEWARD_INSTALL_DIR  install directory; default /usr/local/bin
set -eu

REPO="codesteward-ai/codesteward"
BASE_URL="https://github.com/${REPO}/releases"
INSTALL_DIR="${CODESTEWARD_INSTALL_DIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"

log() {
	printf '%s\n' "$*" >&2
}

fatal() {
	log "error: $*"
	exit 1
}

# --- detect OS ---
os_raw="$(uname -s)"
case "$os_raw" in
	Linux) OS="linux" ;;
	Darwin) OS="darwin" ;;
	MINGW* | MSYS* | CYGWIN* | Windows_NT) OS="windows" ;;
	*) fatal "unsupported operating system: $os_raw" ;;
esac

# --- detect arch ---
arch_raw="$(uname -m)"
case "$arch_raw" in
	x86_64 | amd64) ARCH="amd64" ;;
	arm64 | aarch64) ARCH="arm64" ;;
	*) fatal "unsupported architecture: $arch_raw" ;;
esac

EXT=""
if [ "$OS" = "windows" ]; then
	EXT=".exe"
fi

ASSET="codesteward_${OS}_${ARCH}${EXT}"

if [ "$VERSION" = "latest" ]; then
	ASSET_URL="${BASE_URL}/latest/download/${ASSET}"
	SUMS_URL="${BASE_URL}/latest/download/SHA256SUMS"
else
	ASSET_URL="${BASE_URL}/download/${VERSION}/${ASSET}"
	SUMS_URL="${BASE_URL}/download/${VERSION}/SHA256SUMS"
fi

# --- pick a downloader ---
download() {
	# download <url> <dest>
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1" -o "$2"
	elif command -v wget >/dev/null 2>&1; then
		wget -q "$1" -O "$2"
	else
		fatal "need curl or wget to download binaries"
	fi
}

tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t codesteward)"
trap 'rm -rf "$tmpdir"' EXIT INT TERM

log "downloading ${ASSET} (${VERSION})..."
download "$ASSET_URL" "$tmpdir/$ASSET" || fatal "failed to download $ASSET_URL"

# --- verify checksum when possible ---
sha_tool=""
if command -v sha256sum >/dev/null 2>&1; then
	sha_tool="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
	sha_tool="shasum -a 256"
fi

if [ -n "$sha_tool" ]; then
	if download "$SUMS_URL" "$tmpdir/SHA256SUMS" 2>/dev/null; then
		expected="$(grep " $ASSET\$" "$tmpdir/SHA256SUMS" 2>/dev/null | awk '{print $1}' | head -n 1)"
		if [ -n "$expected" ]; then
			actual="$($sha_tool "$tmpdir/$ASSET" | awk '{print $1}')"
			if [ "$expected" != "$actual" ]; then
				fatal "checksum mismatch for $ASSET (expected $expected, got $actual)"
			fi
			log "checksum verified."
		else
			log "warning: $ASSET not listed in SHA256SUMS; skipping verification"
		fi
	else
		log "warning: could not download SHA256SUMS; skipping checksum verification"
	fi
else
	log "warning: no sha256sum/shasum available; skipping checksum verification"
fi

chmod +x "$tmpdir/$ASSET"

dest="${INSTALL_DIR}/codesteward${EXT}"

# --- install (no auto-sudo) ---
if mkdir -p "$INSTALL_DIR" 2>/dev/null && [ -w "$INSTALL_DIR" ]; then
	mv "$tmpdir/$ASSET" "$dest"
else
	log "error: cannot write to ${INSTALL_DIR}."
	log "Re-run with elevated privileges, e.g.:"
	log "  sudo VERSION=${VERSION} CODESTEWARD_INSTALL_DIR=${INSTALL_DIR} sh $0"
	log "or set CODESTEWARD_INSTALL_DIR to a writable directory (e.g. \$HOME/.local/bin)."
	exit 1
fi

log "installed codesteward to ${dest}"
"$dest" version || true
