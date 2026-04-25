package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/rengotaku/claude-usage-tracker/internal/config"
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

	if err := writeConfig(r); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func writeConfig(r setup.Result) error {
	cfgPath := config.DefaultPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Load existing config to preserve user settings.
	existing, _ := config.Load(cfgPath)

	if r.HasReset {
		existing.WeeklyResetDay = r.WeeklyResetDay.String()
		existing.WeeklyResetHour = r.WeeklyResetHour
	}

	out, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Printf("Wrote %s\n", cfgPath)
	if r.Plan != "" {
		fmt.Printf("  plan:              %s\n", r.Plan)
	}
	if r.HasReset {
		fmt.Printf("  weekly_reset_day:  %s\n", r.WeeklyResetDay)
		fmt.Printf("  weekly_reset_hour: %d\n", r.WeeklyResetHour)
	}
	return nil
}
