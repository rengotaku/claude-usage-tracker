package report_test

import (
	"testing"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/jsonl"
	"github.com/rengotaku/claude-usage-tracker/internal/report"
)

func TestComputeMonthly_ReturnsNilOutsideFirstWeek(t *testing.T) {
	now := fixedTime(2026, 4, 27, 10) // day 27 — not first week
	got := report.ComputeMonthly(nil, now)
	if got != nil {
		t.Errorf("expected nil outside first week, got %+v", got)
	}
}

func TestComputeMonthly_ReturnsDataInFirstWeek(t *testing.T) {
	now := fixedTime(2026, 5, 3, 10) // day 3 — first week
	entries := []jsonl.UsageEntry{
		{
			Timestamp:    time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
			Model:        "claude-sonnet-4-6",
			InputTokens:  1_000_000,
			OutputTokens: 500_000,
		},
		{
			Timestamp:    time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
			Model:        "claude-opus-4-7",
			InputTokens:  200_000,
			OutputTokens: 100_000,
		},
	}
	got := report.ComputeMonthly(entries, now)
	if got == nil {
		t.Fatal("expected monthly data in first week, got nil")
	}
	if got.Label != "2026-04" {
		t.Errorf("expected label 2026-04, got %s", got.Label)
	}
	if got.TotalTokens != 1_800_000 {
		t.Errorf("expected 1800000 total tokens, got %d", got.TotalTokens)
	}
	if got.ByModel["Sonnet"] != 1_500_000 {
		t.Errorf("expected 1500000 for Sonnet, got %d", got.ByModel["Sonnet"])
	}
	if got.ByModel["Opus"] != 300_000 {
		t.Errorf("expected 300000 for Opus, got %d", got.ByModel["Opus"])
	}
}

func TestComputeMonthly_ExcludesOutOfRangeEntries(t *testing.T) {
	now := fixedTime(2026, 5, 1, 10) // day 1 — first week
	entries := []jsonl.UsageEntry{
		{
			Timestamp:   time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), // too old
			Model:       "claude-sonnet-4-6",
			InputTokens: 999_999,
		},
		{
			Timestamp:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), // current month — excluded
			Model:       "claude-sonnet-4-6",
			InputTokens: 999_999,
		},
		{
			Timestamp:   time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC), // in prev month
			Model:       "claude-haiku-4-5",
			InputTokens: 100_000,
		},
	}
	got := report.ComputeMonthly(entries, now)
	if got == nil {
		t.Fatal("expected monthly data, got nil")
	}
	if got.TotalTokens != 100_000 {
		t.Errorf("expected 100000, got %d", got.TotalTokens)
	}
}

func TestComputeMonthly_ZeroEntriesInFirstWeek(t *testing.T) {
	now := fixedTime(2026, 5, 3, 10)
	got := report.ComputeMonthly(nil, now)
	if got == nil {
		t.Fatal("expected MonthlyData even with no entries, got nil")
	}
	if got.TotalTokens != 0 {
		t.Errorf("expected 0 total tokens, got %d", got.TotalTokens)
	}
	if len(got.ByModel) != 0 {
		t.Errorf("expected empty ByModel, got %v", got.ByModel)
	}
}

func TestComputeMonthly_JanuaryPrevMonth(t *testing.T) {
	now := fixedTime(2026, 1, 5, 10) // January first week — prev month is December 2025
	entries := []jsonl.UsageEntry{
		{
			Timestamp:   time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC),
			Model:       "claude-sonnet-4-6",
			InputTokens: 500_000,
		},
	}
	got := report.ComputeMonthly(entries, now)
	if got == nil {
		t.Fatal("expected monthly data, got nil")
	}
	if got.Label != "2025-12" {
		t.Errorf("expected label 2025-12, got %s", got.Label)
	}
}
