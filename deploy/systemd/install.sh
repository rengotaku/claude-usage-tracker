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
CGO_ENABLED=0 go build -o "$BIN_DIR/claude-usage-tracker-setup" ./cmd/setup
CGO_ENABLED=0 go build -o "$BIN_DIR/claude-usage-tracker-report" ./cmd/report

echo "Installing systemd units..."
cp "$SCRIPT_DIR/claude-usage-tracker.service"        "$UNIT_DIR/"
cp "$SCRIPT_DIR/claude-usage-tracker.timer"          "$UNIT_DIR/"
cp "$SCRIPT_DIR/claude-usage-report.service"         "$UNIT_DIR/"
cp "$SCRIPT_DIR/claude-usage-report.timer"           "$UNIT_DIR/"

systemctl --user daemon-reload
systemctl --user enable --now claude-usage-tracker.timer
systemctl --user enable --now claude-usage-report.timer

echo "Done. Verify with: systemctl --user list-timers"
