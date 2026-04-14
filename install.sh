#!/usr/bin/env bash
set -euo pipefail

REPO="forechoandlook/gjlues"
BINARY="gjlues"
INSTALL_DIR="/usr/local/bin"
BIN_PATH="$INSTALL_DIR/$BINARY"

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
esac

case "$OS" in
  darwin) OS="darwin" ;;
  linux)  OS="linux" ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

# Find latest release
LATEST=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | head -1 | sed 's/.*"v\([^"]*\)".*/\1/')
if [ -z "$LATEST" ]; then
  echo "Failed to fetch latest release"
  exit 1
fi

# Check if already installed
if [ -f "$BIN_PATH" ]; then
  CURRENT=$("$BIN_PATH" version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)
  if [ "$CURRENT" = "$LATEST" ]; then
    echo "gjlues v${LATEST} is already installed. Nothing to do."
    exit 0
  fi
  echo "Updating from v${CURRENT:-unknown} to v${LATEST}..."
else
  echo "Installing gjlues v${LATEST} for ${OS}/${ARCH}..."
fi

# Build download URL
ASSET="gjlues_${LATEST}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${LATEST}/${ASSET}"

# Create temp dir
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# Download and extract
curl -sL "$URL" -o "${TMPDIR}/${ASSET}"
tar -xzf "${TMPDIR}/${ASSET}" -C "$TMPDIR"

# Install
chmod +x "$TMPDIR/$BINARY"
sudo mv "$TMPDIR/$BINARY" "$BIN_PATH"

echo "Installed v${LATEST} to $BIN_PATH"
echo "Run 'gjlues version' to verify"
