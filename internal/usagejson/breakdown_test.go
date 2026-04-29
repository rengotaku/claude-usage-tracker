package usagejson

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/rengotaku/claude-usage-tracker/internal/blocks"
)

func TestFromBlocksMapsAllFields(t *testing.T) {
	got := FromBlocks(blocks.TokenBreakdown{
		Input:         1,
		Output:        2,
		CacheCreation: 3,
		CacheRead:     4,
	})
	want := TokenBreakdown{Input: 1, Output: 2, CacheCreation: 3, CacheRead: 4}
	if got != want {
		t.Fatalf("FromBlocks: got %+v, want %+v", got, want)
	}
}

func TestTokenBreakdownJSONKeys(t *testing.T) {
	b, err := json.Marshal(TokenBreakdown{Input: 10, Output: 20, CacheCreation: 30, CacheRead: 40})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]int
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := map[string]int{
		"input":          10,
		"output":         20,
		"cache_creation": 30,
		"cache_read":     40,
	}
	if !reflect.DeepEqual(raw, want) {
		t.Fatalf("JSON keys: got %v, want %v", raw, want)
	}
}

func TestMapFromBlocksReturnsNilForEmpty(t *testing.T) {
	if got := MapFromBlocks(nil); got != nil {
		t.Fatalf("nil input: got %v, want nil", got)
	}
	if got := MapFromBlocks(map[string]blocks.TokenBreakdown{}); got != nil {
		t.Fatalf("empty input: got %v, want nil", got)
	}
}

func TestMapFromBlocksConvertsEntries(t *testing.T) {
	in := map[string]blocks.TokenBreakdown{
		"sonnet": {Input: 1, Output: 2, CacheCreation: 3, CacheRead: 4},
		"opus":   {Input: 5, Output: 6, CacheCreation: 7, CacheRead: 8},
	}
	got := MapFromBlocks(in)
	want := map[string]TokenBreakdown{
		"sonnet": {Input: 1, Output: 2, CacheCreation: 3, CacheRead: 4},
		"opus":   {Input: 5, Output: 6, CacheCreation: 7, CacheRead: 8},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MapFromBlocks: got %+v, want %+v", got, want)
	}
}
