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
	t.Setenv("CLAUDE_USAGE_TRACKER_LOG_DIR", "")
	t.Setenv("CLAUDE_USAGE_TRACKER_PLAN_LIMIT", "")

	cfg := service.ConfigFromEnv()
	if cfg.SessionLimit != 0 {
		t.Errorf("expected default session limit 0, got %d", cfg.SessionLimit)
	}
	if cfg.LogDir == "" {
		t.Error("expected non-empty log dir")
	}
}

func TestConfigFromEnv_EnvOverride(t *testing.T) {
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

func writeJSONL(t *testing.T, dir string, ts time.Time) {
	t.Helper()
	line := `{"type":"assistant","uuid":"u1","timestamp":"` + ts.Format(time.RFC3339) + `","message":{"id":"msg1","model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":200,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}` + "\n"
	path := filepath.Join(dir, "test.jsonl")
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
}
