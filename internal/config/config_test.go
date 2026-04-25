package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rengotaku/claude-usage-tracker/internal/config"
)

func TestLoad_FileNotExist(t *testing.T) {
	cfg, err := config.Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	d := config.Defaults()
	if cfg.WeeklyResetDay != d.WeeklyResetDay {
		t.Errorf("WeeklyResetDay: want %s, got %s", d.WeeklyResetDay, cfg.WeeklyResetDay)
	}
	if cfg.WeeklyResetHour != d.WeeklyResetHour {
		t.Errorf("WeeklyResetHour: want %d, got %d", d.WeeklyResetHour, cfg.WeeklyResetHour)
	}
}

func TestLoad_PartialOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("weekly_reset_day: Thursday\nweekly_reset_hour: 7\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WeeklyResetDay != "Thursday" {
		t.Errorf("WeeklyResetDay: want Thursday, got %s", cfg.WeeklyResetDay)
	}
	if cfg.WeeklyResetHour != 7 {
		t.Errorf("WeeklyResetHour: want 7, got %d", cfg.WeeklyResetHour)
	}
	d := config.Defaults()
	if cfg.LogDir != d.LogDir {
		t.Errorf("LogDir should fall back to default, got %s", cfg.LogDir)
	}
}

func TestLoad_HomeTilde(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("log_dir: ~/mydir\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "mydir")
	if cfg.LogDir != want {
		t.Errorf("LogDir: want %s, got %s", want, cfg.LogDir)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(":::invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
