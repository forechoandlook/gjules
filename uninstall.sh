#!/usr/bin/env bash
set -euo pipefail

BINARY="gjlues"
INSTALL_DIR="/usr/local/bin"
BIN_PATH="$INSTALL_DIR/$BINARY"

if [ ! -f "$BIN_PATH" ]; then
  echo "$BINARY not found at $BIN_PATH"
  exit 1
fi

sudo rm -f "$BIN_PATH"
echo "Uninstalled $BINARY from $BIN_PATH"
