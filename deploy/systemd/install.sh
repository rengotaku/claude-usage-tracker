#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="$HOME/.local/bin"
UNIT_DIR="$HOME/.config/systemd/user"

mkdir -p "$BIN_DIR" "$UNIT_DIR"

echo "Building binaries..."
cd "$SCRIPT_DIR/../.."
CGO_ENABLED=0 go build -o "$BIN_DIR/claude-usage-tracker-snapshot" ./cmd/snapshot
CGO_ENABLED=0 go build -o "$BIN_DIR/claude-usage-tracker-current" ./cmd/current

echo "Installing systemd units..."
cp "$SCRIPT_DIR/claude-usage-tracker.service" "$UNIT_DIR/"
cp "$SCRIPT_DIR/claude-usage-tracker.timer"   "$UNIT_DIR/"

systemctl --user daemon-reload
systemctl --user enable --now claude-usage-tracker.timer

echo "Done. Verify with: systemctl --user list-timers"
