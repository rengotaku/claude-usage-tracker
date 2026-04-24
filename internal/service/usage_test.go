package service_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/service"
)

func TestCompute_NoEntries(t *testing.T) {
	dir := t.TempDir()
	cfg := service.Config{LogDir: dir, SessionLimit: 1_000_000}

	result, err := service.Compute(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SessionTokens != 0 {
		t.Errorf("expected 0 session tokens, got %d", result.SessionTokens)
	}
	if result.SessionRatio != 0 {
		t.Errorf("expected 0 ratio, got %f", result.SessionRatio)
	}
	if result.ActiveBlock != nil {
		t.Error("expected no active block")
	}
}

func TestCompute_ActiveBlock(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	writeJSONL(t, dir, now)

	cfg := service.Config{LogDir: dir, SessionLimit: 1_000_000}
	result, err := service.Compute(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ActiveBlock == nil {
		t.Fatal("expected active block")
	}
	// input(100) + output(200) = 300
	if result.SessionTokens != 300 {
		t.Errorf("expected 300 session tokens, got %d", result.SessionTokens)
	}
	want := float64(300) / float64(1_000_000)
	if result.SessionRatio != want {
		t.Errorf("expected ratio %f, got %f", want, result.SessionRatio)
	}
}

func TestCompute_WeeklyTokens(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	writeJSONL(t, dir, now)

	cfg := service.Config{LogDir: dir}
	result, err := service.Compute(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 300 tokens total, all Sonnet
	if result.WeeklyTokens != 300 {
		t.Errorf("expected 300 weekly tokens, got %d", result.WeeklyTokens)
	}
	if result.WeeklySonnetTokens != 300 {
		t.Errorf("expected 300 weekly sonnet tokens, got %d", result.WeeklySonnetTokens)
	}
}

func TestCompute_WeeklyLimit(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, time.Now().UTC())

	cfg := service.Config{LogDir: dir, WeeklyLimit: 1_000_000, WeeklySonnetLimit: 500_000}
	result, err := service.Compute(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantWeekly := float64(300) / float64(1_000_000)
	if result.WeeklyRatio != wantWeekly {
		t.Errorf("expected weekly ratio %f, got %f", wantWeekly, result.WeeklyRatio)
	}
	wantSonnet := float64(300) / float64(500_000)
	if result.WeeklySonnetRatio != wantSonnet {
		t.Errorf("expected sonnet ratio %f, got %f", wantSonnet, result.WeeklySonnetRatio)
	}
}

func TestConfigFromEnv_Defaults(t *testing.T) {
	// Isolate HOME so credentials detection cannot interfere.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_USAGE_TRACKER_LOG_DIR", "")
	t.Setenv("CLAUDE_USAGE_TRACKER_PLAN_LIMIT", "")
	t.Setenv("CLAUDE_USAGE_TRACKER_WEEKLY_RESET_DAY", "")
	t.Setenv("CLAUDE_USAGE_TRACKER_WEEKLY_RESET_HOUR", "")

	cfg := service.ConfigFromEnv()
	if cfg.SessionLimit != 0 {
		t.Errorf("expected default session limit 0, got %d", cfg.SessionLimit)
	}
	if cfg.LogDir == "" {
		t.Error("expected non-empty log dir")
	}
	if cfg.DetectedTier != "" {
		t.Errorf("expected empty DetectedTier, got %s", cfg.DetectedTier)
	}
	if cfg.SessionLimitFromEnv {
		t.Error("expected SessionLimitFromEnv=false when env unset")
	}
	if cfg.WeeklyResetDay != time.Tuesday {
		t.Errorf("expected default reset day Tuesday, got %s", cfg.WeeklyResetDay)
	}
	if cfg.WeeklyResetHour != 17 {
		t.Errorf("expected default reset hour 17, got %d", cfg.WeeklyResetHour)
	}
}

func TestConfigFromEnv_WeeklyResetOverride(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_USAGE_TRACKER_WEEKLY_RESET_DAY", "Wednesday")
	t.Setenv("CLAUDE_USAGE_TRACKER_WEEKLY_RESET_HOUR", "9")

	cfg := service.ConfigFromEnv()
	if cfg.WeeklyResetDay != time.Wednesday {
		t.Errorf("expected Wednesday, got %s", cfg.WeeklyResetDay)
	}
	if cfg.WeeklyResetHour != 9 {
		t.Errorf("expected 9, got %d", cfg.WeeklyResetHour)
	}
}

func TestConfigFromEnv_DetectsPlanWhenEnvUnset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_USAGE_TRACKER_PLAN_LIMIT", "")
	writeCredentials(t, home, `{"claudeAiOauth":{"rateLimitTier":"default_claude_max_5x"}}`)

	cfg := service.ConfigFromEnv()
	if cfg.DetectedTier != "default_claude_max_5x" {
		t.Errorf("expected detected tier max_5x, got %s", cfg.DetectedTier)
	}
	if cfg.SessionLimit != 45_000_000 {
		t.Errorf("expected session limit 45M (Max 5x), got %d", cfg.SessionLimit)
	}
	if cfg.WeeklyLimit != 833_000_000 {
		t.Errorf("expected weekly limit 833M (Max 5x), got %d", cfg.WeeklyLimit)
	}
	if cfg.WeeklySonnetLimit != 695_000_000 {
		t.Errorf("expected weekly sonnet limit 695M (Max 5x), got %d", cfg.WeeklySonnetLimit)
	}
	if cfg.SessionLimitFromEnv {
		t.Error("expected SessionLimitFromEnv=false (detected, not env)")
	}
}

func TestConfigFromEnv_WeeklyEnvWinsOverDetection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_USAGE_TRACKER_WEEKLY_LIMIT", "900000000")
	t.Setenv("CLAUDE_USAGE_TRACKER_WEEKLY_SONNET_LIMIT", "700000000")
	writeCredentials(t, home, `{"claudeAiOauth":{"rateLimitTier":"default_claude_max_5x"}}`)

	cfg := service.ConfigFromEnv()
	if cfg.WeeklyLimit != 900_000_000 {
		t.Errorf("expected env weekly 900M, got %d", cfg.WeeklyLimit)
	}
	if cfg.WeeklySonnetLimit != 700_000_000 {
		t.Errorf("expected env sonnet 700M, got %d", cfg.WeeklySonnetLimit)
	}
}

func TestConfigFromEnv_UnmappedTierLeavesWeeklyZero(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_USAGE_TRACKER_WEEKLY_LIMIT", "")
	t.Setenv("CLAUDE_USAGE_TRACKER_WEEKLY_SONNET_LIMIT", "")
	// Pro weekly limits are not in the map yet.
	writeCredentials(t, home, `{"claudeAiOauth":{"rateLimitTier":"default_claude_pro"}}`)

	cfg := service.ConfigFromEnv()
	if cfg.WeeklyLimit != 0 {
		t.Errorf("expected weekly limit 0 for unmapped tier, got %d", cfg.WeeklyLimit)
	}
	if cfg.WeeklySonnetLimit != 0 {
		t.Errorf("expected weekly sonnet limit 0 for unmapped tier, got %d", cfg.WeeklySonnetLimit)
	}
	// Session limit for Pro is still populated.
	if cfg.SessionLimit != 19_000_000 {
		t.Errorf("expected session limit 19M (Pro), got %d", cfg.SessionLimit)
	}
}

func TestConfigFromEnv_EnvWinsOverDetection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_USAGE_TRACKER_PLAN_LIMIT", "50000000")
	writeCredentials(t, home, `{"claudeAiOauth":{"rateLimitTier":"default_claude_max_20x"}}`)

	cfg := service.ConfigFromEnv()
	if cfg.SessionLimit != 50_000_000 {
		t.Errorf("expected env-set limit 50M, got %d", cfg.SessionLimit)
	}
	if !cfg.SessionLimitFromEnv {
		t.Error("expected SessionLimitFromEnv=true")
	}
	// Detected tier is still reported for visibility, even when overridden.
	if cfg.DetectedTier != "default_claude_max_20x" {
		t.Errorf("expected detected tier still surfaced, got %s", cfg.DetectedTier)
	}
}

func TestConfigFromEnv_UnknownTier(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_USAGE_TRACKER_PLAN_LIMIT", "")
	writeCredentials(t, home, `{"claudeAiOauth":{"rateLimitTier":"future_tier"}}`)

	cfg := service.ConfigFromEnv()
	if cfg.DetectedTier != "future_tier" {
		t.Errorf("expected tier future_tier, got %s", cfg.DetectedTier)
	}
	if cfg.SessionLimit != 0 {
		t.Errorf("expected 0 session limit for unknown tier, got %d", cfg.SessionLimit)
	}
}

func TestConfigFromEnv_EnvOverride(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_USAGE_TRACKER_LOG_DIR", "/tmp/logs")
	t.Setenv("CLAUDE_USAGE_TRACKER_PLAN_LIMIT", "53000000")
	t.Setenv("CLAUDE_USAGE_TRACKER_WEEKLY_LIMIT", "1000000000")
	t.Setenv("CLAUDE_USAGE_TRACKER_WEEKLY_SONNET_LIMIT", "500000000")

	cfg := service.ConfigFromEnv()
	if cfg.LogDir != "/tmp/logs" {
		t.Errorf("expected /tmp/logs, got %s", cfg.LogDir)
	}
	if cfg.SessionLimit != 53_000_000 {
		t.Errorf("expected 53000000, got %d", cfg.SessionLimit)
	}
	if cfg.WeeklyLimit != 1_000_000_000 {
		t.Errorf("expected 1000000000, got %d", cfg.WeeklyLimit)
	}
	if cfg.WeeklySonnetLimit != 500_000_000 {
		t.Errorf("expected 500000000, got %d", cfg.WeeklySonnetLimit)
	}
}

func writeCredentials(t *testing.T, home, body string) {
	t.Helper()
	credDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(credDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(credDir, ".credentials.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeJSONL(t *testing.T, dir string, ts time.Time) {
	t.Helper()
	line := `{"type":"assistant","uuid":"u1","timestamp":"` + ts.Format(time.RFC3339) + `","message":{"id":"msg1","model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":200,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}` + "\n"
	path := filepath.Join(dir, "test.jsonl")
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
}
