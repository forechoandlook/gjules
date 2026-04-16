#!/usr/bin/env bash
set -euo pipefail

REPO="forechoandlook/gjules"
INSTALL_DIR="$HOME/.local/bin"

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
  mingw*|msys*|cygwin*) OS="windows" ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

# Find latest release version from lightweight asset, with fallback for older releases
LATEST=$(curl -fsSL "https://github.com/${REPO}/releases/latest/download/VERSION" 2>/dev/null | tr -d '\r' | tr -d '\n' || true)
if [ -z "$LATEST" ]; then
  LATEST=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | head -1 | sed 's/.*"v\([^"]*\)".*/\1/')
fi
if [ -z "$LATEST" ]; then
  echo "Failed to fetch latest release"
  exit 1
fi

# Build download URL and binary name
if [ "$OS" = "windows" ]; then
  ASSET="gjules_${LATEST}_windows_${ARCH}.zip"
  BINARY="gjules.exe"
else
  ASSET="gjules_${LATEST}_${OS}_${ARCH}.tar.gz"
  BINARY="gjules"
fi
BIN_PATH="$INSTALL_DIR/$BINARY"
URL="https://github.com/${REPO}/releases/download/v${LATEST}/${ASSET}"

# Check if already installed
if [ -f "$BIN_PATH" ]; then
  CURRENT=$("$BIN_PATH" version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)
  if [ "$CURRENT" = "$LATEST" ]; then
    echo "gjules v${LATEST} is already installed. Nothing to do."
    exit 0
  fi
  echo "Updating from v${CURRENT:-unknown} to v${LATEST}..."
else
  echo "Installing gjules v${LATEST} for ${OS}/${ARCH}..."
fi

# Create temp dir
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# Download and extract
echo "Downloading $URL..."
if ! curl -sfL "$URL" -o "${TMPDIR}/${ASSET}"; then
  echo "Error: Failed to download $URL. The asset may not be available for your OS/Arch yet."
  exit 1
fi
if [ "$OS" = "windows" ]; then
  unzip -q "${TMPDIR}/${ASSET}" -d "$TMPDIR"
else
  tar -xzf "${TMPDIR}/${ASSET}" -C "$TMPDIR"
fi

# Create install dir if it doesn't exist
mkdir -p "$INSTALL_DIR"

# Install
chmod +x "$TMPDIR/$BINARY"
mv "$TMPDIR/$BINARY" "$BIN_PATH"

echo "Installed v${LATEST} to $BIN_PATH"

# Check if INSTALL_DIR is in PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  echo "Warning: $INSTALL_DIR is not in your PATH."
  echo "You may need to add it to your shell profile (e.g., ~/.bashrc or ~/.zshrc):"
  echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
fi

echo "Run 'gjules version' to verify"
