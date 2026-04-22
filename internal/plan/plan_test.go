package plan_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rengotaku/claude-usage-tracker/internal/plan"
)

func TestDetectTierAt_Max5x(t *testing.T) {
	path := writeCredentials(t, `{"claudeAiOauth":{"rateLimitTier":"default_claude_max_5x"}}`)

	tier, err := plan.DetectTierAt(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != "default_claude_max_5x" {
		t.Errorf("expected default_claude_max_5x, got %s", tier)
	}
}

func TestDetectTierAt_Pro(t *testing.T) {
	path := writeCredentials(t, `{"claudeAiOauth":{"rateLimitTier":"default_claude_pro"}}`)

	tier, err := plan.DetectTierAt(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != "default_claude_pro" {
		t.Errorf("expected default_claude_pro, got %s", tier)
	}
}

func TestDetectTierAt_MissingFile(t *testing.T) {
	tier, err := plan.DetectTierAt(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err == nil {
		t.Error("expected error for missing file")
	}
	if tier != "" {
		t.Errorf("expected empty tier, got %s", tier)
	}
}

func TestDetectTierAt_NoTierField(t *testing.T) {
	path := writeCredentials(t, `{"claudeAiOauth":{}}`)

	tier, err := plan.DetectTierAt(path)
	if err == nil {
		t.Error("expected error when rateLimitTier missing")
	}
	if tier != "" {
		t.Errorf("expected empty tier, got %s", tier)
	}
}

func TestDetectTierAt_InvalidJSON(t *testing.T) {
	path := writeCredentials(t, `not json`)

	_, err := plan.DetectTierAt(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSessionLimitForTier(t *testing.T) {
	tests := []struct {
		tier string
		want int
	}{
		{"default_claude_pro", 19_000_000},
		{"default_claude_max_5x", 45_000_000},
		{"default_claude_max_20x", 220_000_000},
		{"unknown_tier", 0},
		{"", 0},
	}
	for _, tt := range tests {
		if got := plan.SessionLimitForTier(tt.tier); got != tt.want {
			t.Errorf("SessionLimitForTier(%q) = %d, want %d", tt.tier, got, tt.want)
		}
	}
}

func TestWeeklyLimitForTier(t *testing.T) {
	tests := []struct {
		tier string
		want int
	}{
		{"default_claude_max_5x", 833_000_000},
		// Pro and Max 20x weekly limits are not yet confirmed from /usage
		// web values; callers must rely on env vars for those tiers.
		{"default_claude_pro", 0},
		{"default_claude_max_20x", 0},
		{"unknown_tier", 0},
		{"", 0},
	}
	for _, tt := range tests {
		if got := plan.WeeklyLimitForTier(tt.tier); got != tt.want {
			t.Errorf("WeeklyLimitForTier(%q) = %d, want %d", tt.tier, got, tt.want)
		}
	}
}

func TestWeeklySonnetLimitForTier(t *testing.T) {
	tests := []struct {
		tier string
		want int
	}{
		{"default_claude_max_5x", 695_000_000},
		{"default_claude_pro", 0},
		{"default_claude_max_20x", 0},
		{"unknown_tier", 0},
		{"", 0},
	}
	for _, tt := range tests {
		if got := plan.WeeklySonnetLimitForTier(tt.tier); got != tt.want {
			t.Errorf("WeeklySonnetLimitForTier(%q) = %d, want %d", tt.tier, got, tt.want)
		}
	}
}

func TestDetectTier_UsesHomeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	credDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(credDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(credDir, ".credentials.json")
	if err := os.WriteFile(path, []byte(`{"claudeAiOauth":{"rateLimitTier":"default_claude_max_20x"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	tier, err := plan.DetectTier()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != "default_claude_max_20x" {
		t.Errorf("expected default_claude_max_20x, got %s", tier)
	}
}

func writeCredentials(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".credentials.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
