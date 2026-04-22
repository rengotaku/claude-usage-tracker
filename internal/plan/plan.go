// Package plan detects the Claude subscription tier from local credentials
// and exposes known session/weekly token limits per tier.
//
// The tier identifier is read from ~/.claude/.credentials.json's
// claudeAiOauth.rateLimitTier field. Numeric limits are derived from the
// web /usage dashboard's percentage display (Anthropic does not publish
// exact numbers) and may drift over time; env vars in the service package
// always take precedence. Limits are only populated for tiers whose
// percentages have been observed; others return 0.
package plan

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	TierPro    = "default_claude_pro"
	TierMax5x  = "default_claude_max_5x"
	TierMax20x = "default_claude_max_20x"
)

// sessionLimits map 5-hour session token caps.
// Max 5x value verified against web /usage on 2026-04-22.
// Pro / Max 20x are prior community estimates, not yet re-verified.
var sessionLimits = map[string]int{
	TierPro:    19_000_000,
	TierMax5x:  45_000_000,
	TierMax20x: 220_000_000,
}

// weeklyLimits map all-models weekly token caps.
// Only populated for tiers with /usage-confirmed values.
var weeklyLimits = map[string]int{
	TierMax5x: 833_000_000,
}

// weeklySonnetLimits map Sonnet-only weekly token caps.
var weeklySonnetLimits = map[string]int{
	TierMax5x: 695_000_000,
}

// DetectTier reads ~/.claude/.credentials.json and returns rateLimitTier.
func DetectTier() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return DetectTierAt(filepath.Join(home, ".claude", ".credentials.json"))
}

// DetectTierAt reads a credentials JSON file at the given path and returns
// the rateLimitTier. Returns an error if the file is missing, malformed, or
// the tier field is absent.
func DetectTierAt(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var creds struct {
		ClaudeAIOauth struct {
			RateLimitTier string `json:"rateLimitTier"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", err
	}
	if creds.ClaudeAIOauth.RateLimitTier == "" {
		return "", errors.New("rateLimitTier not found in credentials")
	}
	return creds.ClaudeAIOauth.RateLimitTier, nil
}

// SessionLimitForTier returns the 5-hour session token limit for a known
// tier, or 0 if the tier is unknown.
func SessionLimitForTier(tier string) int {
	return sessionLimits[tier]
}

// WeeklyLimitForTier returns the all-models weekly token limit for a known
// tier, or 0 if the tier is unknown / not yet measured.
func WeeklyLimitForTier(tier string) int {
	return weeklyLimits[tier]
}

// WeeklySonnetLimitForTier returns the Sonnet-only weekly token limit for a
// known tier, or 0 if the tier is unknown / not yet measured.
func WeeklySonnetLimitForTier(tier string) int {
	return weeklySonnetLimits[tier]
}
