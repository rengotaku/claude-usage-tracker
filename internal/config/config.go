// Package config loads application settings from ~/.config/claude-usage-tracker/config.yaml.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all application settings.
type Config struct {
	LogDir            string `yaml:"log_dir"`
	DB                string `yaml:"db"`
	PlanLimit         int    `yaml:"plan_limit"`
	WeeklyLimit       int    `yaml:"weekly_limit"`
	WeeklySonnetLimit int    `yaml:"weekly_sonnet_limit"`
	WeeklyResetDay    string `yaml:"weekly_reset_day"`
	WeeklyResetHour   int    `yaml:"weekly_reset_hour"`
	Port              string `yaml:"port"`
}

// DefaultPath returns the config file path. CLAUDE_USAGE_TRACKER_CONFIG env var
// overrides the default ~/.config/claude-usage-tracker/config.yaml.
func DefaultPath() string {
	if v := os.Getenv("CLAUDE_USAGE_TRACKER_CONFIG"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "claude-usage-tracker", "config.yaml")
}

// Load reads the config file at path. If the file does not exist, defaults are returned.
// Missing fields fall back to defaults.
func Load(path string) (Config, error) {
	cfg := Defaults()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}

	var file Config
	if err := yaml.Unmarshal(data, &file); err != nil {
		return cfg, err
	}

	if file.LogDir != "" {
		cfg.LogDir = expandHome(file.LogDir)
	}
	if file.DB != "" {
		cfg.DB = expandHome(file.DB)
	}
	if file.PlanLimit != 0 {
		cfg.PlanLimit = file.PlanLimit
	}
	if file.WeeklyLimit != 0 {
		cfg.WeeklyLimit = file.WeeklyLimit
	}
	if file.WeeklySonnetLimit != 0 {
		cfg.WeeklySonnetLimit = file.WeeklySonnetLimit
	}
	if file.WeeklyResetDay != "" {
		cfg.WeeklyResetDay = file.WeeklyResetDay
	}
	if file.WeeklyResetHour != 0 {
		cfg.WeeklyResetHour = file.WeeklyResetHour
	}
	if file.Port != "" {
		cfg.Port = file.Port
	}
	return cfg, nil
}

// Defaults returns the built-in default configuration.
func Defaults() Config {
	home, _ := os.UserHomeDir()
	return Config{
		LogDir:          filepath.Join(home, ".claude", "projects"),
		DB:              filepath.Join(home, ".local", "share", "claude-usage-tracker", "snapshots.db"),
		WeeklyResetDay:  "Tuesday",
		WeeklyResetHour: 17,
		Port:            "8080",
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
