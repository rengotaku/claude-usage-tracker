// Package plan detects the Claude subscription tier from local credentials.
//
// The tier identifier is read from ~/.claude/.credentials.json's
// claudeAiOauth.rateLimitTier field. Token limits are not embedded here —
// configure plan_limit / weekly_limit / weekly_sonnet_limit in
// ~/.config/claude-usage-tracker/config.yaml instead.
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
