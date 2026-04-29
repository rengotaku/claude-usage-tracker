// Package cache persists and retrieves UsageResult to avoid re-parsing JSONL on every call.
package cache

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/blocks"
	"github.com/rengotaku/claude-usage-tracker/internal/service"
)

type entry struct {
	CachedAt         time.Time             `json:"cached_at"`
	SessionTokens    int                   `json:"session_tokens"`
	SessionBreakdown blocks.TokenBreakdown `json:"session_breakdown"`
	SessionLimit     int                   `json:"session_limit"`
	SessionRatio     float64               `json:"session_ratio"`
	SessionEndsAt    *time.Time            `json:"session_ends_at,omitempty"`
	ActiveBlock      *blocks.Block         `json:"active_block,omitempty"`

	WeeklyTokens         int                              `json:"weekly_tokens"`
	WeeklyLimit          int                              `json:"weekly_limit"`
	WeeklyRatio          float64                          `json:"weekly_ratio"`
	WeeklySonnetTokens   int                              `json:"weekly_sonnet_tokens"`
	WeeklySonnetLimit    int                              `json:"weekly_sonnet_limit"`
	WeeklySonnetRatio    float64                          `json:"weekly_sonnet_ratio"`
	WeeklyResetsAt       time.Time                        `json:"weekly_resets_at"`
	WeeklyModelBreakdown map[string]blocks.TokenBreakdown `json:"weekly_model_breakdown,omitempty"`
}

// Load reads a cached UsageResult from path. Returns nil if the cache is missing,
// unreadable, or older than ttl.
func Load(path string, ttl time.Duration) (*service.UsageResult, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, nil
	}

	if time.Since(e.CachedAt) > ttl {
		return nil, nil
	}

	return &service.UsageResult{
		SessionTokens:        e.SessionTokens,
		SessionBreakdown:     e.SessionBreakdown,
		SessionLimit:         e.SessionLimit,
		SessionRatio:         e.SessionRatio,
		SessionEndsAt:        e.SessionEndsAt,
		ActiveBlock:          e.ActiveBlock,
		WeeklyTokens:         e.WeeklyTokens,
		WeeklyLimit:          e.WeeklyLimit,
		WeeklyRatio:          e.WeeklyRatio,
		WeeklySonnetTokens:   e.WeeklySonnetTokens,
		WeeklySonnetLimit:    e.WeeklySonnetLimit,
		WeeklySonnetRatio:    e.WeeklySonnetRatio,
		WeeklyResetsAt:       e.WeeklyResetsAt,
		WeeklyModelBreakdown: e.WeeklyModelBreakdown,
	}, nil
}

// Save writes result to path as a JSON cache entry.
func Save(path string, result *service.UsageResult) error {
	e := entry{
		CachedAt:             time.Now(),
		SessionTokens:        result.SessionTokens,
		SessionBreakdown:     result.SessionBreakdown,
		SessionLimit:         result.SessionLimit,
		SessionRatio:         result.SessionRatio,
		SessionEndsAt:        result.SessionEndsAt,
		ActiveBlock:          result.ActiveBlock,
		WeeklyTokens:         result.WeeklyTokens,
		WeeklyLimit:          result.WeeklyLimit,
		WeeklyRatio:          result.WeeklyRatio,
		WeeklySonnetTokens:   result.WeeklySonnetTokens,
		WeeklySonnetLimit:    result.WeeklySonnetLimit,
		WeeklySonnetRatio:    result.WeeklySonnetRatio,
		WeeklyResetsAt:       result.WeeklyResetsAt,
		WeeklyModelBreakdown: result.WeeklyModelBreakdown,
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
