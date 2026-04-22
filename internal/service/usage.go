package service

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/blocks"
	"github.com/rengotaku/claude-usage-tracker/internal/jsonl"
	"github.com/rengotaku/claude-usage-tracker/internal/plan"
)

var jst = time.FixedZone("JST", 9*60*60)

// Config holds runtime configuration for usage computation.
type Config struct {
	LogDir            string
	SessionLimit      int // 5-hour block limit (0 = unknown)
	WeeklyLimit       int // weekly all-models limit (0 = unknown)
	WeeklySonnetLimit int // weekly Sonnet-only limit (0 = unknown)

	// DetectedTier is the rateLimitTier read from ~/.claude/.credentials.json
	// (empty if the file is missing or unreadable). Surfaced for logging so
	// users can confirm the detected plan even when env vars override it.
	DetectedTier string
	// SessionLimitFromEnv reports whether SessionLimit came from the env var
	// (true) or from the tier map fallback (false).
	SessionLimitFromEnv bool
}

// ConfigFromEnv builds Config from environment variables, falling back to
// tier-based session limits detected from ~/.claude/.credentials.json when
// CLAUDE_USAGE_TRACKER_PLAN_LIMIT is not set. Env vars always win.
func ConfigFromEnv() Config {
	logDir := os.Getenv("CLAUDE_USAGE_TRACKER_LOG_DIR")
	if logDir == "" {
		home, _ := os.UserHomeDir()
		logDir = home + "/.claude/projects"
	}

	envLimit := envInt("CLAUDE_USAGE_TRACKER_PLAN_LIMIT", 0)
	envWeekly := envInt("CLAUDE_USAGE_TRACKER_WEEKLY_LIMIT", 0)
	envWeeklySonnet := envInt("CLAUDE_USAGE_TRACKER_WEEKLY_SONNET_LIMIT", 0)
	detectedTier, _ := plan.DetectTier()

	sessionLimit := envLimit
	if sessionLimit == 0 && detectedTier != "" {
		sessionLimit = plan.SessionLimitForTier(detectedTier)
	}
	weeklyLimit := envWeekly
	if weeklyLimit == 0 && detectedTier != "" {
		weeklyLimit = plan.WeeklyLimitForTier(detectedTier)
	}
	weeklySonnetLimit := envWeeklySonnet
	if weeklySonnetLimit == 0 && detectedTier != "" {
		weeklySonnetLimit = plan.WeeklySonnetLimitForTier(detectedTier)
	}

	return Config{
		LogDir:              logDir,
		SessionLimit:        sessionLimit,
		WeeklyLimit:         weeklyLimit,
		WeeklySonnetLimit:   weeklySonnetLimit,
		DetectedTier:        detectedTier,
		SessionLimitFromEnv: envLimit > 0,
	}
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// UsageResult holds session and weekly usage metrics.
type UsageResult struct {
	// Current 5-hour session block
	SessionTokens int
	SessionLimit  int
	SessionRatio  float64 // 0 if limit unknown
	SessionEndsAt *time.Time
	ActiveBlock   *blocks.Block

	// Weekly (since last Friday 08:00 JST)
	WeeklyTokens       int
	WeeklyLimit        int
	WeeklyRatio        float64 // 0 if limit unknown
	WeeklySonnetTokens int
	WeeklySonnetLimit  int
	WeeklySonnetRatio  float64 // 0 if limit unknown
	WeeklyResetsAt     time.Time
}

// Compute parses JSONL logs and returns session + weekly usage.
func Compute(cfg Config) (*UsageResult, error) {
	entries, err := jsonl.Parse(cfg.LogDir)
	if err != nil {
		return nil, fmt.Errorf("parse logs: %w", err)
	}

	bs := blocks.Build(entries)
	active := blocks.ActiveBlock(bs)

	result := &UsageResult{
		SessionLimit:      cfg.SessionLimit,
		WeeklyLimit:       cfg.WeeklyLimit,
		WeeklySonnetLimit: cfg.WeeklySonnetLimit,
		WeeklyResetsAt:    nextWeeklyReset(),
	}

	if active != nil {
		result.SessionTokens = active.TotalTokens
		result.ActiveBlock = active
		end := active.EndTime
		result.SessionEndsAt = &end
		if cfg.SessionLimit > 0 {
			result.SessionRatio = float64(active.TotalTokens) / float64(cfg.SessionLimit)
		}
	}

	weekStart := lastWeeklyReset()
	for _, e := range entries {
		if e.Timestamp.Before(weekStart) {
			continue
		}
		t := totalTokens(e)
		result.WeeklyTokens += t
		if isSonnet(e.Model) {
			result.WeeklySonnetTokens += t
		}
	}
	if cfg.WeeklyLimit > 0 {
		result.WeeklyRatio = float64(result.WeeklyTokens) / float64(cfg.WeeklyLimit)
	}
	if cfg.WeeklySonnetLimit > 0 {
		result.WeeklySonnetRatio = float64(result.WeeklySonnetTokens) / float64(cfg.WeeklySonnetLimit)
	}

	return result, nil
}

// lastWeeklyReset returns the most recent Friday 08:00 JST.
func lastWeeklyReset() time.Time {
	now := time.Now().In(jst)
	// weekday: Monday=1 ... Friday=5 ... Sunday=0
	daysSinceFriday := (int(now.Weekday()) - int(time.Friday) + 7) % 7
	reset := time.Date(now.Year(), now.Month(), now.Day()-daysSinceFriday, 8, 0, 0, 0, jst)
	if now.Before(reset) {
		reset = reset.AddDate(0, 0, -7)
	}
	return reset
}

// nextWeeklyReset returns the next Friday 08:00 JST.
func nextWeeklyReset() time.Time {
	last := lastWeeklyReset()
	return last.AddDate(0, 0, 7)
}

func totalTokens(e jsonl.UsageEntry) int {
	return e.InputTokens + e.OutputTokens + e.CacheCreationInputTokens + e.CacheReadInputTokens
}

func isSonnet(model string) bool {
	return strings.Contains(strings.ToLower(model), "sonnet")
}
