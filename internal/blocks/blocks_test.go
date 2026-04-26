package blocks_test

import (
	"testing"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/blocks"
	"github.com/rengotaku/claude-usage-tracker/internal/jsonl"
)

func entry(ts time.Time, input, output, cacheCreate, cacheRead int) jsonl.UsageEntry {
	return jsonl.UsageEntry{
		Timestamp:                ts,
		InputTokens:              input,
		OutputTokens:             output,
		CacheCreationInputTokens: cacheCreate,
		CacheReadInputTokens:     cacheRead,
	}
}

func entryWithModel(ts time.Time, model string, input, output, cacheCreate, cacheRead int) jsonl.UsageEntry {
	return jsonl.UsageEntry{
		Timestamp:                ts,
		Model:                    model,
		InputTokens:              input,
		OutputTokens:             output,
		CacheCreationInputTokens: cacheCreate,
		CacheReadInputTokens:     cacheRead,
	}
}

func utc(year int, month time.Month, day, hour, min int) time.Time {
	return time.Date(year, month, day, hour, min, 0, 0, time.UTC)
}

func TestBuild_EmptyEntries(t *testing.T) {
	result := blocks.Build(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestBuild_SingleEntry(t *testing.T) {
	entries := []jsonl.UsageEntry{
		entry(utc(2026, 4, 19, 10, 30), 10, 20, 5, 3),
	}
	result := blocks.Build(entries)
	if len(result) != 1 {
		t.Fatalf("expected 1 block, got %d", len(result))
	}
	b := result[0]
	// block starts at exact first-entry timestamp (aligned with Claude's session window)
	want := utc(2026, 4, 19, 10, 30)
	if !b.StartTime.Equal(want) {
		t.Errorf("StartTime: want %v, got %v", want, b.StartTime)
	}
	if !b.EndTime.Equal(want.Add(5 * time.Hour)) {
		t.Errorf("EndTime: want %v, got %v", want.Add(5*time.Hour), b.EndTime)
	}
	if b.TotalTokens != 38 {
		t.Errorf("TotalTokens: want 38, got %d", b.TotalTokens)
	}
	if b.EntryCount != 1 {
		t.Errorf("EntryCount: want 1, got %d", b.EntryCount)
	}
}

func TestBuild_EntriesWithinSameBlock(t *testing.T) {
	entries := []jsonl.UsageEntry{
		entry(utc(2026, 4, 19, 10, 0), 10, 10, 0, 0),
		entry(utc(2026, 4, 19, 12, 0), 20, 20, 0, 0),
		entry(utc(2026, 4, 19, 14, 59), 5, 5, 0, 0),
	}
	result := blocks.Build(entries)
	if len(result) != 1 {
		t.Fatalf("expected 1 block, got %d", len(result))
	}
	if result[0].EntryCount != 3 {
		t.Errorf("EntryCount: want 3, got %d", result[0].EntryCount)
	}
	if result[0].TotalTokens != 70 {
		t.Errorf("TotalTokens: want 70, got %d", result[0].TotalTokens)
	}
}

func TestBuild_ExactlyAtBlockBoundary(t *testing.T) {
	// 10:00 → block [10:00, 15:00); entry at 15:00 starts new block
	entries := []jsonl.UsageEntry{
		entry(utc(2026, 4, 19, 10, 0), 10, 0, 0, 0),
		entry(utc(2026, 4, 19, 15, 0), 20, 0, 0, 0),
	}
	result := blocks.Build(entries)
	if len(result) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result))
	}
	if result[0].TotalTokens != 10 {
		t.Errorf("block[0] TotalTokens: want 10, got %d", result[0].TotalTokens)
	}
	if result[1].TotalTokens != 20 {
		t.Errorf("block[1] TotalTokens: want 20, got %d", result[1].TotalTokens)
	}
}

func TestBuild_GapBetweenBlocks(t *testing.T) {
	entries := []jsonl.UsageEntry{
		entry(utc(2026, 4, 19, 10, 0), 10, 0, 0, 0),
		// 24-hour gap
		entry(utc(2026, 4, 20, 10, 0), 20, 0, 0, 0),
	}
	result := blocks.Build(entries)
	if len(result) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result))
	}
}

func TestBuild_UnsortedEntriesAreSorted(t *testing.T) {
	entries := []jsonl.UsageEntry{
		entry(utc(2026, 4, 19, 14, 0), 20, 0, 0, 0),
		entry(utc(2026, 4, 19, 10, 0), 10, 0, 0, 0),
	}
	result := blocks.Build(entries)
	if len(result) != 1 {
		t.Fatalf("expected 1 block, got %d", len(result))
	}
	if result[0].EntryCount != 2 {
		t.Errorf("EntryCount: want 2, got %d", result[0].EntryCount)
	}
}

func TestBuild_ModelBreakdown(t *testing.T) {
	entries := []jsonl.UsageEntry{
		entryWithModel(utc(2026, 4, 19, 10, 0), "claude-sonnet-4-6", 100, 200, 0, 0),
		entryWithModel(utc(2026, 4, 19, 11, 0), "claude-haiku-4-5", 50, 30, 10, 5),
		entryWithModel(utc(2026, 4, 19, 12, 0), "claude-sonnet-4-6", 20, 10, 0, 0),
	}
	result := blocks.Build(entries)
	if len(result) != 1 {
		t.Fatalf("expected 1 block, got %d", len(result))
	}
	b := result[0]

	sonnet, ok := b.ModelBreakdown["claude-sonnet-4-6"]
	if !ok {
		t.Fatal("expected claude-sonnet-4-6 in ModelBreakdown")
	}
	if sonnet.Input != 120 || sonnet.Output != 210 {
		t.Errorf("sonnet: want Input=120,Output=210 got Input=%d,Output=%d", sonnet.Input, sonnet.Output)
	}

	haiku, ok := b.ModelBreakdown["claude-haiku-4-5"]
	if !ok {
		t.Fatal("expected claude-haiku-4-5 in ModelBreakdown")
	}
	if haiku.Input != 50 || haiku.Output != 30 || haiku.CacheCreation != 10 || haiku.CacheRead != 5 {
		t.Errorf("haiku: want Input=50,Output=30,CacheCreation=10,CacheRead=5, got Input=%d,Output=%d,CacheCreation=%d,CacheRead=%d",
			haiku.Input, haiku.Output, haiku.CacheCreation, haiku.CacheRead)
	}
}

func TestBuild_ModelBreakdown_EmptyModel(t *testing.T) {
	entries := []jsonl.UsageEntry{
		entry(utc(2026, 4, 19, 10, 0), 10, 20, 0, 0),
	}
	result := blocks.Build(entries)
	if len(result) != 1 {
		t.Fatalf("expected 1 block, got %d", len(result))
	}
	if len(result[0].ModelBreakdown) != 0 {
		t.Errorf("expected empty ModelBreakdown for entry with no model, got %v", result[0].ModelBreakdown)
	}
}

func TestActiveBlock_NilWhenNoActive(t *testing.T) {
	bs := []blocks.Block{
		{StartTime: utc(2020, 1, 1, 0, 0), EndTime: utc(2020, 1, 1, 5, 0), IsActive: false},
	}
	if blocks.ActiveBlock(bs) != nil {
		t.Error("expected nil ActiveBlock")
	}
}

func TestActiveBlock_ReturnsActive(t *testing.T) {
	bs := []blocks.Block{
		{StartTime: utc(2020, 1, 1, 0, 0), EndTime: utc(2020, 1, 1, 5, 0), IsActive: false},
		{StartTime: utc(2020, 1, 1, 5, 0), EndTime: utc(2020, 1, 1, 10, 0), IsActive: true},
	}
	got := blocks.ActiveBlock(bs)
	if got == nil {
		t.Fatal("expected non-nil ActiveBlock")
	}
	if !got.IsActive {
		t.Error("returned block is not active")
	}
}
