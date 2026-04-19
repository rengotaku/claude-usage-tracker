package service

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rengotaku/claude-usage-tracker/internal/blocks"
	"github.com/rengotaku/claude-usage-tracker/internal/jsonl"
)

const defaultPlanLimit = 200_000_000

// Config holds runtime configuration for usage computation.
type Config struct {
	LogDir    string
	PlanLimit int
}

// ConfigFromEnv builds Config from environment variables with defaults.
func ConfigFromEnv() Config {
	logDir := os.Getenv("CLAUDE_USAGE_TRACKER_LOG_DIR")
	if logDir == "" {
		home, _ := os.UserHomeDir()
		logDir = home + "/.claude/projects"
	}

	planLimit := defaultPlanLimit
	if v := os.Getenv("CLAUDE_USAGE_TRACKER_PLAN_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			planLimit = n
		}
	}

	return Config{LogDir: logDir, PlanLimit: planLimit}
}

// UsageResult is the computed usage for the current billing block.
type UsageResult struct {
	TokensUsed  int
	PlanLimit   int
	UsageRatio  float64
	ActiveBlock *blocks.Block
}

// Compute parses JSONL logs and returns current block usage.
// If no active block exists, TokensUsed and UsageRatio are zero.
func Compute(cfg Config) (*UsageResult, error) {
	if cfg.PlanLimit <= 0 {
		return nil, fmt.Errorf("plan_limit must be positive")
	}

	entries, err := jsonl.Parse(cfg.LogDir)
	if err != nil {
		return nil, fmt.Errorf("parse logs: %w", err)
	}

	bs := blocks.Build(entries)
	active := blocks.ActiveBlock(bs)

	result := &UsageResult{PlanLimit: cfg.PlanLimit}
	if active != nil {
		result.TokensUsed = active.TotalTokens
		result.UsageRatio = float64(active.TotalTokens) / float64(cfg.PlanLimit)
		result.ActiveBlock = active
	}
	return result, nil
}
