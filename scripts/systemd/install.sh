#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="$HOME/.local/bin"
UNIT_DIR="$HOME/.config/systemd/user"

mkdir -p "$BIN_DIR" "$UNIT_DIR"

echo "Installing report script..."
install -m 0755 "$SCRIPT_DIR/../report_usage.py" "$BIN_DIR/claude-usage-report.py"

echo "Installing systemd units..."
cp "$SCRIPT_DIR/claude-usage-report.service" "$UNIT_DIR/"
cp "$SCRIPT_DIR/claude-usage-report.timer"   "$UNIT_DIR/"

systemctl --user daemon-reload
systemctl --user enable --now claude-usage-report.timer

echo "Done. Verify with: systemctl --user list-timers"
echo "Test run:          $BIN_DIR/claude-usage-report.py --dry-run"
