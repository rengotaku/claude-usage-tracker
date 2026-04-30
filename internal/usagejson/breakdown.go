// Package usagejson contains JSON DTOs that are shared between the HTTP API
// (internal/server) and CLI commands (cmd/current). Keeping these in one place
// avoids drift between API responses and CLI --json output.
package usagejson

import "github.com/rengotaku/claude-usage-tracker/internal/blocks"

// TokenBreakdown is the JSON wire format for blocks.TokenBreakdown.
//
// It is intentionally separate from blocks.TokenBreakdown: the latter is also
// persisted to SQLite via json.Marshal without struct tags, so adding snake_case
// tags there would silently break readback of existing rows.
type TokenBreakdown struct {
	Input         int `json:"input"`
	Output        int `json:"output"`
	CacheCreation int `json:"cache_creation"`
	CacheRead     int `json:"cache_read"`
}

// FromBlocks converts a blocks.TokenBreakdown to its JSON DTO form.
func FromBlocks(b blocks.TokenBreakdown) TokenBreakdown {
	return TokenBreakdown{
		Input:         b.Input,
		Output:        b.Output,
		CacheCreation: b.CacheCreation,
		CacheRead:     b.CacheRead,
	}
}

// MapFromBlocks converts a per-model breakdown map to its JSON DTO form.
// Returns nil for empty input so callers can rely on omitempty behavior.
func MapFromBlocks(m map[string]blocks.TokenBreakdown) map[string]TokenBreakdown {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]TokenBreakdown, len(m))
	for k, v := range m {
		out[k] = FromBlocks(v)
	}
	return out
}
