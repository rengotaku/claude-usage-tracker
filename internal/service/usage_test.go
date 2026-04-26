package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/config"
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

func TestCompute_WeeklyModelBreakdown(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	writeMixedJSONL(t, dir, now)

	cfg := service.Config{LogDir: dir}
	result, err := service.Compute(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.WeeklyModelBreakdown) != 2 {
		t.Fatalf("expected 2 models in WeeklyModelBreakdown, got %d", len(result.WeeklyModelBreakdown))
	}
	sonnet, ok := result.WeeklyModelBreakdown["claude-sonnet-4-6"]
	if !ok {
		t.Fatal("expected claude-sonnet-4-6 in WeeklyModelBreakdown")
	}
	if sonnet.Input != 100 || sonnet.Output != 200 {
		t.Errorf("sonnet: want Input=100,Output=200, got Input=%d,Output=%d", sonnet.Input, sonnet.Output)
	}
	haiku, ok := result.WeeklyModelBreakdown["claude-haiku-4-5"]
	if !ok {
		t.Fatal("expected claude-haiku-4-5 in WeeklyModelBreakdown")
	}
	if haiku.Input != 50 || haiku.Output != 30 {
		t.Errorf("haiku: want Input=50,Output=30, got Input=%d,Output=%d", haiku.Input, haiku.Output)
	}
}

func TestConfigFrom_Defaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	d := config.Defaults()
	cfg := service.ConfigFrom(d)

	if cfg.SessionLimit != 0 {
		t.Errorf("expected default session limit 0, got %d", cfg.SessionLimit)
	}
	if cfg.WeeklyLimit != 0 {
		t.Errorf("expected default weekly limit 0, got %d", cfg.WeeklyLimit)
	}
	if cfg.WeeklySonnetLimit != 0 {
		t.Errorf("expected default weekly sonnet limit 0, got %d", cfg.WeeklySonnetLimit)
	}
	if cfg.LogDir == "" {
		t.Error("expected non-empty log dir")
	}
	if cfg.DetectedTier != "" {
		t.Errorf("expected empty DetectedTier, got %s", cfg.DetectedTier)
	}
	if cfg.WeeklyResetDay != time.Tuesday {
		t.Errorf("expected default reset day Tuesday, got %s", cfg.WeeklyResetDay)
	}
	if cfg.WeeklyResetHour != 17 {
		t.Errorf("expected default reset hour 17, got %d", cfg.WeeklyResetHour)
	}
}

func TestConfigFrom_WeeklyResetOverride(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c := config.Defaults()
	c.WeeklyResetDay = "Wednesday"
	c.WeeklyResetHour = 9

	cfg := service.ConfigFrom(c)
	if cfg.WeeklyResetDay != time.Wednesday {
		t.Errorf("expected Wednesday, got %s", cfg.WeeklyResetDay)
	}
	if cfg.WeeklyResetHour != 9 {
		t.Errorf("expected 9, got %d", cfg.WeeklyResetHour)
	}
}

func TestConfigFrom_TierDetected(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCredentials(t, home, `{"claudeAiOauth":{"rateLimitTier":"default_claude_max_5x"}}`)

	c := config.Defaults()
	cfg := service.ConfigFrom(c)
	if cfg.DetectedTier != "default_claude_max_5x" {
		t.Errorf("expected detected tier max_5x, got %s", cfg.DetectedTier)
	}
	// limits come from config only — not auto-populated from tier
	if cfg.SessionLimit != 0 {
		t.Errorf("expected session limit 0 (not set in config), got %d", cfg.SessionLimit)
	}
	if cfg.WeeklyLimit != 0 {
		t.Errorf("expected weekly limit 0 (not set in config), got %d", cfg.WeeklyLimit)
	}
	if cfg.WeeklySonnetLimit != 0 {
		t.Errorf("expected weekly sonnet limit 0 (not set in config), got %d", cfg.WeeklySonnetLimit)
	}
}

func TestConfigFrom_ConfigLimitsUsed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCredentials(t, home, `{"claudeAiOauth":{"rateLimitTier":"default_claude_max_5x"}}`)

	c := config.Defaults()
	c.PlanLimit = 90_000_000
	c.WeeklyLimit = 1_260_000_000
	c.WeeklySonnetLimit = 1_300_000_000

	cfg := service.ConfigFrom(c)
	if cfg.SessionLimit != 90_000_000 {
		t.Errorf("expected 90M, got %d", cfg.SessionLimit)
	}
	if cfg.WeeklyLimit != 1_260_000_000 {
		t.Errorf("expected 1260M, got %d", cfg.WeeklyLimit)
	}
	if cfg.WeeklySonnetLimit != 1_300_000_000 {
		t.Errorf("expected 1300M, got %d", cfg.WeeklySonnetLimit)
	}
}

func TestConfigFrom_Override(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c := config.Defaults()
	c.LogDir = "/tmp/logs"
	c.PlanLimit = 53_000_000
	c.WeeklyLimit = 1_000_000_000
	c.WeeklySonnetLimit = 500_000_000

	cfg := service.ConfigFrom(c)
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

func TestValidateConfig_AllSet(t *testing.T) {
	cfg := service.Config{
		SessionLimit:      90_000_000,
		WeeklyLimit:       1_260_000_000,
		WeeklySonnetLimit: 1_300_000_000,
	}
	if err := service.ValidateConfig(cfg); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateConfig_MissingAll(t *testing.T) {
	cfg := service.Config{}
	err := service.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for missing limits")
	}
	msg := err.Error()
	for _, field := range []string{"plan_limit", "weekly_limit", "weekly_sonnet_limit"} {
		if !contains(msg, field) {
			t.Errorf("expected error to mention %q, got: %s", field, msg)
		}
	}
}

func TestValidateConfig_MissingPartial(t *testing.T) {
	cfg := service.Config{SessionLimit: 90_000_000}
	err := service.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for missing weekly limits")
	}
	if contains(err.Error(), "plan_limit") {
		t.Errorf("plan_limit should not be in error, got: %s", err.Error())
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsHelper(s, sub))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
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

func writeMixedJSONL(t *testing.T, dir string, ts time.Time) {
	t.Helper()
	lines := []string{
		`{"type":"assistant","uuid":"u1","timestamp":"` + ts.Format(time.RFC3339) + `","message":{"id":"msg1","model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":200,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`,
		`{"type":"assistant","uuid":"u2","timestamp":"` + ts.Add(time.Minute).Format(time.RFC3339) + `","message":{"id":"msg2","model":"claude-haiku-4-5","usage":{"input_tokens":50,"output_tokens":30,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`,
	}
	path := filepath.Join(dir, "test.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
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
