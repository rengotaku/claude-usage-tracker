#!/usr/bin/env bash
set -euo pipefail

PLIST_DIR="$HOME/Library/LaunchAgents"
PLIST_NAME="com.user.claude-usage-tracker"

launchctl unload "$PLIST_DIR/$PLIST_NAME.plist" 2>/dev/null || true
rm -f "$PLIST_DIR/$PLIST_NAME.plist"
rm -f "$HOME/.local/bin/claude-usage-tracker-snapshot"
rm -f "$HOME/.local/bin/claude-usage-tracker-current"

echo "Uninstalled."
