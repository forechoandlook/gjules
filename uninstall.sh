#!/usr/bin/env bash
set -euo pipefail

BINARY="gjlues"
# Detect OS to handle .exe extension
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  mingw*|msys*|cygwin*) BINARY="gjlues.exe" ;;
esac

INSTALL_DIR="$HOME/.local/bin"
BIN_PATH="$INSTALL_DIR/$BINARY"

if [ ! -f "$BIN_PATH" ]; then
  # Fallback: check without .exe if not found, or vice versa
  if [ "$BINARY" = "gjlues" ] && [ -f "$INSTALL_DIR/gjlues.exe" ]; then
    BIN_PATH="$INSTALL_DIR/gjlues.exe"
  elif [ "$BINARY" = "gjlues.exe" ] && [ -f "$INSTALL_DIR/gjlues" ]; then
    BIN_PATH="$INSTALL_DIR/gjlues"
  else
    echo "gjlues not found at $INSTALL_DIR"
    exit 0
  fi
fi

rm -f "$BIN_PATH"
echo "Uninstalled gjlues from $BIN_PATH"
