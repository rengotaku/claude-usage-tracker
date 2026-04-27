#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="$HOME/.local/bin"
PLIST_DIR="$HOME/Library/LaunchAgents"
PLIST_NAME="com.user.claude-usage-tracker"
DB_DIR="$HOME/.local/share/claude-usage-tracker"

mkdir -p "$BIN_DIR" "$PLIST_DIR" "$DB_DIR"

echo "Building binaries..."
cd "$SCRIPT_DIR/../.."
CGO_ENABLED=0 go build -o "$BIN_DIR/claude-usage-tracker-snapshot" ./cmd/snapshot
CGO_ENABLED=0 go build -o "$BIN_DIR/claude-usage-tracker-current" ./cmd/current
CGO_ENABLED=0 go build -o "$BIN_DIR/claude-usage-tracker-setup" ./cmd/setup
CGO_ENABLED=0 go build -o "$BIN_DIR/claude-usage-tracker-report" ./cmd/report

echo "Installing launchd plists..."
REPORT_PLIST_NAME="com.user.claude-usage-report"
sed "s|{{HOME}}|$HOME|g" "$SCRIPT_DIR/$PLIST_NAME.plist" > "$PLIST_DIR/$PLIST_NAME.plist"
sed "s|{{HOME}}|$HOME|g" "$SCRIPT_DIR/$REPORT_PLIST_NAME.plist" > "$PLIST_DIR/$REPORT_PLIST_NAME.plist"

launchctl unload "$PLIST_DIR/$PLIST_NAME.plist" 2>/dev/null || true
launchctl load "$PLIST_DIR/$PLIST_NAME.plist"
launchctl unload "$PLIST_DIR/$REPORT_PLIST_NAME.plist" 2>/dev/null || true
launchctl load "$PLIST_DIR/$REPORT_PLIST_NAME.plist"

echo "Done. Verify with: launchctl list $PLIST_NAME && launchctl list $REPORT_PLIST_NAME"
