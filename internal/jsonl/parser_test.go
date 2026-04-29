package jsonl_test

import (
	"testing"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/jsonl"
)

func TestParse_ReturnsUsageEntries(t *testing.T) {
	entries, err := jsonl.Parse("testdata")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	// sample.jsonl has 2 unique assistant messages with usage (msg_001 deduped)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestParse_FieldValues(t *testing.T) {
	entries, err := jsonl.Parse("testdata")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(entries) < 1 {
		t.Fatal("no entries returned")
	}

	e := entries[0]
	if e.MessageID != "msg_001" {
		t.Errorf("MessageID: want msg_001, got %s", e.MessageID)
	}
	if e.Model != "claude-sonnet-4-6" {
		t.Errorf("Model: want claude-sonnet-4-6, got %s", e.Model)
	}
	if e.InputTokens != 10 {
		t.Errorf("InputTokens: want 10, got %d", e.InputTokens)
	}
	if e.OutputTokens != 20 {
		t.Errorf("OutputTokens: want 20, got %d", e.OutputTokens)
	}
	if e.CacheCreationInputTokens != 100 {
		t.Errorf("CacheCreationInputTokens: want 100, got %d", e.CacheCreationInputTokens)
	}
	if e.CacheReadInputTokens != 50 {
		t.Errorf("CacheReadInputTokens: want 50, got %d", e.CacheReadInputTokens)
	}
	want := time.Date(2026, 4, 18, 8, 0, 1, 0, time.UTC)
	if !e.Timestamp.Equal(want) {
		t.Errorf("Timestamp: want %v, got %v", want, e.Timestamp)
	}
}

func TestParse_Deduplication(t *testing.T) {
	// msg_001 appears twice in testdata/sample.jsonl; should only count once
	entries, err := jsonl.Parse("testdata")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	seen := make(map[string]int)
	for _, e := range entries {
		seen[e.MessageID]++
	}
	for id, count := range seen {
		if count > 1 {
			t.Errorf("duplicate entry for MessageID %s: count=%d", id, count)
		}
	}
}

func TestParse_NonExistentDir(t *testing.T) {
	_, err := jsonl.Parse("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

func TestUsageEntry_TotalTokens(t *testing.T) {
	tests := []struct {
		name string
		e    jsonl.UsageEntry
		want int
	}{
		{"all fields", jsonl.UsageEntry{InputTokens: 10, OutputTokens: 20, CacheCreationInputTokens: 100, CacheReadInputTokens: 50}, 180},
		{"zeros", jsonl.UsageEntry{}, 0},
		{"only input", jsonl.UsageEntry{InputTokens: 5}, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.TotalTokens(); got != tt.want {
				t.Errorf("TotalTokens() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParse_SkipsMalformedLines(t *testing.T) {
	// sample.jsonl contains one malformed line; Parse should not return error
	entries, err := jsonl.Parse("testdata")
	if err != nil {
		t.Fatalf("Parse should not error on malformed lines, got: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected entries despite malformed lines")
	}
}
