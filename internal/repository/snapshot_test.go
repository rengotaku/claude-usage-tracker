package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/blocks"
	"github.com/rengotaku/claude-usage-tracker/internal/repository"
)

func newTestRepo(t *testing.T) *repository.SnapshotRepository {
	t.Helper()
	r, err := repository.NewSnapshotRepository(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

func TestSaveAndLatest(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	end := now.Add(5 * time.Hour)
	s := repository.Snapshot{
		TakenAt:        now,
		BlockStartedAt: now,
		BlockEndedAt:   &end,
		TokensUsed:     1000,
		UsageRatio:     0.5,
	}

	if err := r.Save(ctx, s); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := r.Latest(ctx)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if got == nil {
		t.Fatal("expected snapshot, got nil")
	}

	if !got.TakenAt.Equal(s.TakenAt) {
		t.Errorf("TakenAt: got %v, want %v", got.TakenAt, s.TakenAt)
	}
	if !got.BlockStartedAt.Equal(s.BlockStartedAt) {
		t.Errorf("BlockStartedAt: got %v, want %v", got.BlockStartedAt, s.BlockStartedAt)
	}
	if got.TokensUsed != s.TokensUsed {
		t.Errorf("TokensUsed: got %d, want %d", got.TokensUsed, s.TokensUsed)
	}
	if got.UsageRatio != s.UsageRatio {
		t.Errorf("UsageRatio: got %f, want %f", got.UsageRatio, s.UsageRatio)
	}
	if got.BlockEndedAt == nil || !got.BlockEndedAt.Equal(*s.BlockEndedAt) {
		t.Errorf("BlockEndedAt: got %v, want %v", got.BlockEndedAt, s.BlockEndedAt)
	}
}

func TestLatestEmpty(t *testing.T) {
	r := newTestRepo(t)
	got, err := r.Latest(context.Background())
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestSaveUpsert(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	s := repository.Snapshot{
		TakenAt:        now,
		BlockStartedAt: now,
		TokensUsed:     100,
		UsageRatio:     0.1,
	}
	if err := r.Save(ctx, s); err != nil {
		t.Fatalf("save 1: %v", err)
	}

	s.TokensUsed = 200
	s.UsageRatio = 0.2
	if err := r.Save(ctx, s); err != nil {
		t.Fatalf("save 2 (upsert): %v", err)
	}

	got, err := r.Latest(ctx)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if got.TokensUsed != 200 {
		t.Errorf("TokensUsed after upsert: got %d, want 200", got.TokensUsed)
	}
}

func TestListBetween(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		s := repository.Snapshot{
			TakenAt:        base.Add(time.Duration(i) * time.Hour),
			BlockStartedAt: base,
			TokensUsed:     (i + 1) * 100,
			UsageRatio:     float64(i+1) * 0.1,
		}
		if err := r.Save(ctx, s); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}

	from := base.Add(1 * time.Hour)
	to := base.Add(3 * time.Hour)
	got, err := r.ListBetween(ctx, from, to)
	if err != nil {
		t.Fatalf("list between: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len: got %d, want 3", len(got))
	}
	wantTokens := []int{200, 300, 400}
	for i, want := range wantTokens {
		if got[i].TokensUsed != want {
			t.Errorf("got[%d].TokensUsed: got %d, want %d", i, got[i].TokensUsed, want)
		}
	}
}

func TestLatestReturnsNewest(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	older := repository.Snapshot{TakenAt: base, BlockStartedAt: base, TokensUsed: 111, UsageRatio: 0.1}
	newer := repository.Snapshot{TakenAt: base.Add(time.Hour), BlockStartedAt: base, TokensUsed: 999, UsageRatio: 0.9}

	if err := r.Save(ctx, older); err != nil {
		t.Fatalf("save older: %v", err)
	}
	if err := r.Save(ctx, newer); err != nil {
		t.Fatalf("save newer: %v", err)
	}

	got, err := r.Latest(ctx)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if got == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if got.TokensUsed != 999 {
		t.Errorf("Latest should return newest: got TokensUsed=%d, want 999", got.TokensUsed)
	}
}

func TestSaveAndLatest_ModelBreakdown(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	s := repository.Snapshot{
		TakenAt:        now,
		BlockStartedAt: now,
		TokensUsed:     1000,
		UsageRatio:     0.5,
		WeeklyModelBreakdown: map[string]blocks.TokenBreakdown{
			"claude-sonnet-4-6": {Input: 100, Output: 200, CacheCreation: 10, CacheRead: 50},
			"claude-haiku-4-5":  {Input: 50, Output: 30},
		},
	}
	if err := r.Save(ctx, s); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := r.Latest(ctx)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if got == nil {
		t.Fatal("expected snapshot")
	}
	sonnet, ok := got.WeeklyModelBreakdown["claude-sonnet-4-6"]
	if !ok {
		t.Fatal("expected claude-sonnet-4-6 in WeeklyModelBreakdown")
	}
	if sonnet.Input != 100 || sonnet.Output != 200 || sonnet.CacheCreation != 10 || sonnet.CacheRead != 50 {
		t.Errorf("sonnet: want {100 200 10 50}, got %+v", sonnet)
	}
	haiku, ok := got.WeeklyModelBreakdown["claude-haiku-4-5"]
	if !ok {
		t.Fatal("expected claude-haiku-4-5 in WeeklyModelBreakdown")
	}
	if haiku.Input != 50 || haiku.Output != 30 {
		t.Errorf("haiku: want {50 30 0 0}, got %+v", haiku)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/snapshots.db"
	ctx := context.Background()

	r1, err := repository.NewSnapshotRepository(ctx, dbPath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	if err := r1.Save(ctx, repository.Snapshot{
		TakenAt: now, BlockStartedAt: now, TokensUsed: 7, UsageRatio: 0.07,
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	r1.Close()

	// Reopening must re-run migration without error and preserve data.
	r2, err := repository.NewSnapshotRepository(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer r2.Close()
	got, err := r2.Latest(ctx)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if got == nil || got.TokensUsed != 7 {
		t.Fatalf("expected snapshot tokens=7, got %+v", got)
	}
}

func TestNullBlockEndedAt(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	s := repository.Snapshot{
		TakenAt:        now,
		BlockStartedAt: now,
		BlockEndedAt:   nil,
		TokensUsed:     500,
		UsageRatio:     0.25,
	}
	if err := r.Save(ctx, s); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := r.Latest(ctx)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if got.BlockEndedAt != nil {
		t.Errorf("BlockEndedAt: expected nil, got %v", got.BlockEndedAt)
	}
}
