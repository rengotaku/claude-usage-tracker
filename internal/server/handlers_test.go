package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/repository"
	"github.com/rengotaku/claude-usage-tracker/internal/server"
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

func seedSnapshots(t *testing.T, r *repository.SnapshotRepository, snaps ...repository.Snapshot) {
	t.Helper()
	ctx := context.Background()
	for _, s := range snaps {
		if err := r.Save(ctx, s); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body.String())
	}
	return v
}

func TestSnapshotsEndpoint(t *testing.T) {
	r := newTestRepo(t)
	base := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	end := base.Add(5 * time.Hour)
	seedSnapshots(t, r,
		repository.Snapshot{TakenAt: base, BlockStartedAt: base, BlockEndedAt: &end, TokensUsed: 1000, UsageRatio: 0.02, WeeklyTokens: 1000},
		repository.Snapshot{TakenAt: base.Add(time.Hour), BlockStartedAt: base, BlockEndedAt: &end, TokensUsed: 5000, UsageRatio: 0.10, WeeklyTokens: 5000},
	)

	h := server.NewHandler(r, server.Config{SessionLimit: 50000, WeeklyLimit: 500000})
	req := httptest.NewRequest(http.MethodGet, "/usage/snapshots?from=2026-04-14&to=2026-04-15", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	type resp struct {
		Snapshots []struct {
			SessionTokens int     `json:"session_tokens"`
			SessionRatio  float64 `json:"session_ratio"`
		} `json:"snapshots"`
	}
	got := decode[resp](t, rec)
	if len(got.Snapshots) != 2 {
		t.Fatalf("len snapshots: got %d, want 2", len(got.Snapshots))
	}
	if got.Snapshots[1].SessionTokens != 5000 {
		t.Errorf("tokens[1]: got %d, want 5000", got.Snapshots[1].SessionTokens)
	}
}

func TestSnapshotsInvalidFrom(t *testing.T) {
	r := newTestRepo(t)
	h := server.NewHandler(r, server.Config{})
	req := httptest.NewRequest(http.MethodGet, "/usage/snapshots?from=not-a-date", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
}

func TestBlocksEndpointAggregation(t *testing.T) {
	r := newTestRepo(t)
	// Two snapshots in block A (tokens should be MAX), one snapshot in block B.
	blockAStart := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	blockAEnd := blockAStart.Add(5 * time.Hour)
	blockBStart := blockAStart.Add(6 * time.Hour)
	blockBEnd := blockBStart.Add(5 * time.Hour)
	seedSnapshots(t, r,
		repository.Snapshot{TakenAt: blockAStart, BlockStartedAt: blockAStart, BlockEndedAt: &blockAEnd, TokensUsed: 2000, UsageRatio: 0.04, WeeklyTokens: 2000},
		repository.Snapshot{TakenAt: blockAStart.Add(time.Hour), BlockStartedAt: blockAStart, BlockEndedAt: &blockAEnd, TokensUsed: 8000, UsageRatio: 0.16, WeeklyTokens: 8000},
		repository.Snapshot{TakenAt: blockBStart, BlockStartedAt: blockBStart, BlockEndedAt: &blockBEnd, TokensUsed: 3000, UsageRatio: 0.06, WeeklyTokens: 11000},
	)

	h := server.NewHandler(r, server.Config{SessionLimit: 50000, WeeklyLimit: 500000})
	req := httptest.NewRequest(http.MethodGet, "/usage/blocks?from=2026-04-14&to=2026-04-15", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	type blk struct {
		Tokens int     `json:"tokens"`
		Ratio  float64 `json:"ratio"`
	}
	type resp struct {
		Blocks []blk `json:"blocks"`
		Weekly struct {
			TotalTokens int     `json:"total_tokens"`
			Limit       int     `json:"limit"`
			Ratio       float64 `json:"ratio"`
		} `json:"weekly"`
	}
	got := decode[resp](t, rec)
	if len(got.Blocks) != 2 {
		t.Fatalf("len blocks: got %d, want 2", len(got.Blocks))
	}
	if got.Blocks[0].Tokens != 8000 {
		t.Errorf("blocks[0].tokens (MAX): got %d, want 8000", got.Blocks[0].Tokens)
	}
	if got.Blocks[1].Tokens != 3000 {
		t.Errorf("blocks[1].tokens: got %d, want 3000", got.Blocks[1].Tokens)
	}
	if got.Weekly.TotalTokens != 11000 {
		t.Errorf("weekly total: got %d, want 11000", got.Weekly.TotalTokens)
	}
	if got.Weekly.Limit != 500000 {
		t.Errorf("weekly limit: got %d, want 500000", got.Weekly.Limit)
	}
	wantRatio := float64(11000) / float64(500000)
	if got.Weekly.Ratio < wantRatio-0.0001 || got.Weekly.Ratio > wantRatio+0.0001 {
		t.Errorf("weekly ratio: got %f, want %f", got.Weekly.Ratio, wantRatio)
	}
}

func TestDailyEndpoint(t *testing.T) {
	r := newTestRepo(t)
	// Day 1 (UTC 10:00 = JST 19:00 same day 2026-04-14): two blocks
	d1a := time.Date(2026, 4, 14, 1, 0, 0, 0, time.UTC) // JST 10:00
	d1aEnd := d1a.Add(5 * time.Hour)
	d1b := time.Date(2026, 4, 14, 8, 0, 0, 0, time.UTC) // JST 17:00
	d1bEnd := d1b.Add(5 * time.Hour)
	// Day 2 (JST 2026-04-15)
	d2 := time.Date(2026, 4, 15, 1, 0, 0, 0, time.UTC) // JST 10:00
	d2End := d2.Add(5 * time.Hour)

	seedSnapshots(t, r,
		repository.Snapshot{TakenAt: d1a, BlockStartedAt: d1a, BlockEndedAt: &d1aEnd, TokensUsed: 5000, UsageRatio: 0.1},
		repository.Snapshot{TakenAt: d1b, BlockStartedAt: d1b, BlockEndedAt: &d1bEnd, TokensUsed: 3000, UsageRatio: 0.06},
		repository.Snapshot{TakenAt: d2, BlockStartedAt: d2, BlockEndedAt: &d2End, TokensUsed: 2000, UsageRatio: 0.04},
	)

	h := server.NewHandler(r, server.Config{})
	req := httptest.NewRequest(http.MethodGet, "/usage/daily?from=2026-04-14&to=2026-04-16", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	type day struct {
		Date   string `json:"date"`
		Tokens int    `json:"tokens"`
		Blocks int    `json:"blocks"`
	}
	type resp struct {
		Daily []day `json:"daily"`
	}
	got := decode[resp](t, rec)
	if len(got.Daily) != 2 {
		t.Fatalf("len daily: got %d, want 2", len(got.Daily))
	}
	if got.Daily[0].Date != "2026-04-14" || got.Daily[0].Tokens != 8000 || got.Daily[0].Blocks != 2 {
		t.Errorf("daily[0]: got %+v, want {2026-04-14 8000 2}", got.Daily[0])
	}
	if got.Daily[1].Date != "2026-04-15" || got.Daily[1].Tokens != 2000 || got.Daily[1].Blocks != 1 {
		t.Errorf("daily[1]: got %+v, want {2026-04-15 2000 1}", got.Daily[1])
	}
}

func TestSummaryEndpoint(t *testing.T) {
	r := newTestRepo(t)
	now := time.Now().UTC()
	// current week: one block 10000 tokens
	cur := now.Add(-24 * time.Hour)
	curEnd := cur.Add(5 * time.Hour)
	// previous week: one block 5000 tokens
	prev := now.Add(-8 * 24 * time.Hour)
	prevEnd := prev.Add(5 * time.Hour)

	seedSnapshots(t, r,
		repository.Snapshot{TakenAt: cur, BlockStartedAt: cur, BlockEndedAt: &curEnd, TokensUsed: 10000, UsageRatio: 0.2, WeeklyTokens: 10000, WeeklySonnetTokens: 4000},
		repository.Snapshot{TakenAt: prev, BlockStartedAt: prev, BlockEndedAt: &prevEnd, TokensUsed: 5000, UsageRatio: 0.1, WeeklyTokens: 5000},
	)

	h := server.NewHandler(r, server.Config{})
	req := httptest.NewRequest(http.MethodGet, "/usage/summary?window=week", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	type resp struct {
		Window       string  `json:"window"`
		Current      struct{ Tokens int `json:"tokens"` } `json:"current"`
		Previous     struct{ Tokens int `json:"tokens"` } `json:"previous"`
		DeltaRatio   float64 `json:"delta_ratio"`
		WeeklySonnet int     `json:"weekly_sonnet_tokens"`
	}
	got := decode[resp](t, rec)
	if got.Window != "week" {
		t.Errorf("window: got %q, want week", got.Window)
	}
	if got.Current.Tokens != 10000 {
		t.Errorf("current tokens: got %d, want 10000", got.Current.Tokens)
	}
	if got.Previous.Tokens != 5000 {
		t.Errorf("previous tokens: got %d, want 5000", got.Previous.Tokens)
	}
	if got.DeltaRatio != 1.0 {
		t.Errorf("delta: got %f, want 1.0", got.DeltaRatio)
	}
	if got.WeeklySonnet != 4000 {
		t.Errorf("weekly sonnet: got %d, want 4000", got.WeeklySonnet)
	}
}

func TestBlocksFiltersPlaceholderSnapshots(t *testing.T) {
	r := newTestRepo(t)
	realStart := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	realEnd := realStart.Add(5 * time.Hour)
	// Placeholder (no active block): BlockEndedAt = nil, tokens = 0.
	placeholder := time.Date(2026, 4, 14, 16, 30, 0, 0, time.UTC)
	seedSnapshots(t, r,
		repository.Snapshot{TakenAt: realStart, BlockStartedAt: realStart, BlockEndedAt: &realEnd, TokensUsed: 5000, UsageRatio: 0.1},
		repository.Snapshot{TakenAt: placeholder, BlockStartedAt: placeholder, BlockEndedAt: nil, TokensUsed: 0},
	)

	h := server.NewHandler(r, server.Config{})
	req := httptest.NewRequest(http.MethodGet, "/usage/blocks?from=2026-04-14&to=2026-04-15", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	type resp struct {
		Blocks []struct {
			Tokens int `json:"tokens"`
		} `json:"blocks"`
	}
	got := decode[resp](t, rec)
	if len(got.Blocks) != 1 {
		t.Fatalf("len blocks: got %d, want 1 (placeholder must be filtered)", len(got.Blocks))
	}
	if got.Blocks[0].Tokens != 5000 {
		t.Errorf("blocks[0].tokens: got %d, want 5000", got.Blocks[0].Tokens)
	}
}

func TestSummaryInvalidWindow(t *testing.T) {
	r := newTestRepo(t)
	h := server.NewHandler(r, server.Config{})
	req := httptest.NewRequest(http.MethodGet, "/usage/summary?window=decade", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
}

func TestHealthz(t *testing.T) {
	r := newTestRepo(t)
	h := server.NewHandler(r, server.Config{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body: got %q, want ok", rec.Body.String())
	}
}
