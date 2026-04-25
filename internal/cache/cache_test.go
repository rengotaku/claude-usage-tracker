package cache_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/cache"
	"github.com/rengotaku/claude-usage-tracker/internal/service"
)

func sampleResult() *service.UsageResult {
	endsAt := time.Now().Add(time.Hour)
	return &service.UsageResult{
		SessionTokens:      1000,
		SessionLimit:       90_000_000,
		SessionRatio:       0.01,
		SessionEndsAt:      &endsAt,
		WeeklyTokens:       5000,
		WeeklyLimit:        1_260_000_000,
		WeeklyRatio:        0.004,
		WeeklySonnetTokens: 2000,
		WeeklySonnetLimit:  1_300_000_000,
		WeeklySonnetRatio:  0.002,
		WeeklyResetsAt:     time.Now().Add(24 * time.Hour),
	}
}

func TestSaveLoad_HitWithinTTL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	r := sampleResult()

	if err := cache.Save(path, r); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := cache.Load(path, time.Minute)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil {
		t.Fatal("expected cache hit, got nil")
	}
	if got.SessionTokens != r.SessionTokens {
		t.Errorf("session tokens: got %d, want %d", got.SessionTokens, r.SessionTokens)
	}
	if got.WeeklyTokens != r.WeeklyTokens {
		t.Errorf("weekly tokens: got %d, want %d", got.WeeklyTokens, r.WeeklyTokens)
	}
}

func TestLoad_MissWhenExpired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")

	if err := cache.Save(path, sampleResult()); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := cache.Load(path, 0) // TTL=0 → always expired
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != nil {
		t.Error("expected cache miss (expired), got result")
	}
}

func TestLoad_MissWhenFileAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")

	got, err := cache.Load(path, time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for missing cache file")
	}
}

func TestLoad_MissOnCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := cache.Load(path, time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for corrupt cache file")
	}
}
