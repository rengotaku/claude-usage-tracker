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
	cfg := service.Config{LogDir: dir, PlanLimit: 1_000_000}

	result, err := service.Compute(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TokensUsed != 0 {
		t.Errorf("expected 0 tokens, got %d", result.TokensUsed)
	}
	if result.UsageRatio != 0 {
		t.Errorf("expected 0 ratio, got %f", result.UsageRatio)
	}
	if result.ActiveBlock != nil {
		t.Error("expected no active block")
	}
}

func TestCompute_ActiveBlock(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	writeJSONL(t, dir, now)

	cfg := service.Config{LogDir: dir, PlanLimit: 1_000_000}
	result, err := service.Compute(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ActiveBlock == nil {
		t.Fatal("expected active block")
	}
	// input(100) + output(200) = 300
	if result.TokensUsed != 300 {
		t.Errorf("expected 300 tokens, got %d", result.TokensUsed)
	}
	want := float64(300) / float64(1_000_000)
	if result.UsageRatio != want {
		t.Errorf("expected ratio %f, got %f", want, result.UsageRatio)
	}
}

func TestCompute_InvalidPlanLimit(t *testing.T) {
	cfg := service.Config{LogDir: t.TempDir(), PlanLimit: 0}
	_, err := service.Compute(cfg)
	if err == nil {
		t.Error("expected error for zero plan_limit")
	}
}

func TestConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv("CLAUDE_USAGE_TRACKER_LOG_DIR", "")
	t.Setenv("CLAUDE_USAGE_TRACKER_PLAN_LIMIT", "")

	cfg := service.ConfigFromEnv()
	if cfg.PlanLimit != 200_000_000 {
		t.Errorf("expected default plan limit 200_000_000, got %d", cfg.PlanLimit)
	}
	if cfg.LogDir == "" {
		t.Error("expected non-empty log dir")
	}
}

func TestConfigFromEnv_EnvOverride(t *testing.T) {
	t.Setenv("CLAUDE_USAGE_TRACKER_LOG_DIR", "/tmp/logs")
	t.Setenv("CLAUDE_USAGE_TRACKER_PLAN_LIMIT", "500000")

	cfg := service.ConfigFromEnv()
	if cfg.LogDir != "/tmp/logs" {
		t.Errorf("expected /tmp/logs, got %s", cfg.LogDir)
	}
	if cfg.PlanLimit != 500_000 {
		t.Errorf("expected 500000, got %d", cfg.PlanLimit)
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
