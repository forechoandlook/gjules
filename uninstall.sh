#!/usr/bin/env bash
set -euo pipefail

BINARY="gjules"
# Detect OS to handle .exe extension
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  mingw*|msys*|cygwin*) BINARY="gjules.exe" ;;
esac

INSTALL_DIR="$HOME/.local/bin"
BIN_PATH="$INSTALL_DIR/$BINARY"

if [ ! -f "$BIN_PATH" ]; then
  # Fallback: check without .exe if not found, or vice versa
  if [ "$BINARY" = "gjules" ] && [ -f "$INSTALL_DIR/gjules.exe" ]; then
    BIN_PATH="$INSTALL_DIR/gjules.exe"
  elif [ "$BINARY" = "gjules.exe" ] && [ -f "$INSTALL_DIR/gjules" ]; then
    BIN_PATH="$INSTALL_DIR/gjules"
  else
    echo "gjules not found at $INSTALL_DIR"
    exit 0
  fi
fi

rm -f "$BIN_PATH"
echo "Uninstalled gjules from $BIN_PATH"
