// Package plan detects the Claude subscription tier from local credentials
// and exposes the community-known session token limits per tier.
//
// The tier identifier is read from ~/.claude/.credentials.json's
// claudeAiOauth.rateLimitTier field. The numeric session limits are
// community estimates (Anthropic does not publish exact values) and may
// drift over time; env vars in the service package always take precedence.
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

var sessionLimits = map[string]int{
	TierPro:    19_000_000,
	TierMax5x:  88_000_000,
	TierMax20x: 220_000_000,
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
