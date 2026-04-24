package main

import (
	"fmt"
	"io"
	"os"

	"github.com/rengotaku/claude-usage-tracker/internal/setup"
)

func main() {
	fmt.Fprintln(os.Stderr, "Claude Usage Tracker Setup")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "1. Open https://claude.ai/usage in your browser")
	fmt.Fprintln(os.Stderr, "2. Copy all the text on the page and paste it below")
	fmt.Fprintln(os.Stderr, "3. Press Ctrl+D when done")
	fmt.Fprintln(os.Stderr, "")

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading input: %v\n", err)
		os.Exit(1)
	}

	r := setup.ParseWebText(string(data))
	if !r.HasReset && r.Tier == "" {
		fmt.Fprintln(os.Stderr, "ERROR: Could not parse usage data.")
		fmt.Fprintln(os.Stderr, "Please paste the full text from https://claude.ai/usage")
		os.Exit(1)
	}

	printRecommendations(r)
}

func printRecommendations(r setup.Result) {
	if r.Plan != "" {
		fmt.Printf("# Plan: %s\n", r.Plan)
	}
	if r.HasReset {
		fmt.Printf("# Weekly reset: %s %02d:00 (web UI timezone — adjust to JST if needed)\n", r.WeeklyResetDay, r.WeeklyResetHour)
	}
	fmt.Println()

	fmt.Println("# ---- ~/.bashrc or ~/.zshrc ----")
	if r.HasReset {
		fmt.Printf("export CLAUDE_USAGE_TRACKER_WEEKLY_RESET_DAY=%s\n", r.WeeklyResetDay)
		fmt.Printf("export CLAUDE_USAGE_TRACKER_WEEKLY_RESET_HOUR=%d\n", r.WeeklyResetHour)
	}
	fmt.Println()

	fmt.Println("# ---- systemd: deploy/systemd/claude-usage-tracker.service ----")
	if r.HasReset {
		fmt.Printf("Environment=CLAUDE_USAGE_TRACKER_WEEKLY_RESET_DAY=%s\n", r.WeeklyResetDay)
		fmt.Printf("Environment=CLAUDE_USAGE_TRACKER_WEEKLY_RESET_HOUR=%d\n", r.WeeklyResetHour)
	}
	fmt.Println()

	fmt.Println("# ---- launchd: deploy/launchd/com.user.claude-usage-tracker.plist ----")
	if r.HasReset {
		fmt.Printf("<key>CLAUDE_USAGE_TRACKER_WEEKLY_RESET_DAY</key>\n<string>%s</string>\n", r.WeeklyResetDay)
		fmt.Printf("<key>CLAUDE_USAGE_TRACKER_WEEKLY_RESET_HOUR</key>\n<string>%d</string>\n", r.WeeklyResetHour)
	}
}
