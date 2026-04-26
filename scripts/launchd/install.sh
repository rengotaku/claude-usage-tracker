#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="$HOME/.local/bin"
PLIST_DIR="$HOME/Library/LaunchAgents"
PLIST_NAME="com.user.claude-usage-report"
LOG_DIR="$HOME/.local/share/claude-usage-tracker"

mkdir -p "$BIN_DIR" "$PLIST_DIR" "$LOG_DIR"

echo "Installing report script..."
install -m 0755 "$SCRIPT_DIR/../report_usage.sh" "$BIN_DIR/claude-usage-report"

echo "Installing launchd plist..."
sed "s|{{HOME}}|$HOME|g" "$SCRIPT_DIR/$PLIST_NAME.plist" > "$PLIST_DIR/$PLIST_NAME.plist"

launchctl unload "$PLIST_DIR/$PLIST_NAME.plist" 2>/dev/null || true
launchctl load "$PLIST_DIR/$PLIST_NAME.plist"

echo "Done. Verify with: launchctl list $PLIST_NAME"
echo "Test run:          $BIN_DIR/claude-usage-report --dry-run"
