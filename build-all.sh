#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

export PATH="${WAILS_GO_BIN:-/Users/alex/go/bin}:$PATH"
export GOCACHE="${GOCACHE:-/private/tmp/reliure-go-cache}"

WAILS="${WAILS:-wails3}"
MAC_ARCH="${MAC_ARCH:-$(go env GOARCH)}"
WINDOWS_ARCH="${WINDOWS_ARCH:-amd64}"
LINUX_ARCH="${LINUX_ARCH:-amd64}"
LINUX_OUTPUT="${LINUX_OUTPUT:-bin/reliure-linux-${LINUX_ARCH}}"
RELEASE_DIR="${RELEASE_DIR:-bin/releases}"
STAGE_DIR=""

log() {
  printf '\n==> %s\n' "$*"
}

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

need "$WAILS"
need go
need ditto

STAGE_DIR="$(mktemp -d)"
trap 'if [ -n "$STAGE_DIR" ]; then rm -rf "$STAGE_DIR"; fi' EXIT
mkdir -p "$RELEASE_DIR"

log "Building macOS .app (${MAC_ARCH})"
"$WAILS" task package GOOS=darwin ARCH="$MAC_ARCH" -f
log "Zipping macOS .app"
ditto -c -k --sequesterRsrc --keepParent "bin/reliure.app" "$RELEASE_DIR/reliure-macos-${MAC_ARCH}.zip"

log "Building Windows .exe (${WINDOWS_ARCH})"
"$WAILS" task build GOOS=windows ARCH="$WINDOWS_ARCH" -f
log "Zipping Windows .exe"
rm -rf "$STAGE_DIR/windows"
mkdir -p "$STAGE_DIR/windows"
cp "bin/reliure.exe" "$STAGE_DIR/windows/reliure.exe"
ditto -c -k "$STAGE_DIR/windows" "$RELEASE_DIR/reliure-windows-${WINDOWS_ARCH}.zip"

log "Preparing Docker cross-build image for Linux"
need docker
docker build --platform "linux/${LINUX_ARCH}" -t wails-cross -f build/docker/Dockerfile.cross build/docker/

log "Building Linux binary (${LINUX_ARCH})"
"$WAILS" task build GOOS=linux ARCH="$LINUX_ARCH" OUTPUT="$LINUX_OUTPUT" -f
log "Zipping Linux binary"
rm -rf "$STAGE_DIR/linux"
mkdir -p "$STAGE_DIR/linux"
cp "$LINUX_OUTPUT" "$STAGE_DIR/linux/reliure"
chmod +x "$STAGE_DIR/linux/reliure"
ditto -c -k "$STAGE_DIR/linux" "$RELEASE_DIR/reliure-linux-${LINUX_ARCH}.zip"

cat <<EOF

Build complete:
  macOS:   bin/reliure.app
  Windows: bin/reliure.exe
  Linux:   ${LINUX_OUTPUT}

Release zips:
  ${RELEASE_DIR}/reliure-macos-${MAC_ARCH}.zip
  ${RELEASE_DIR}/reliure-windows-${WINDOWS_ARCH}.zip
  ${RELEASE_DIR}/reliure-linux-${LINUX_ARCH}.zip

EOF
