package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/blocks"
	"github.com/rengotaku/claude-usage-tracker/internal/config"
	"github.com/rengotaku/claude-usage-tracker/internal/jsonl"
	"github.com/rengotaku/claude-usage-tracker/internal/plan"
)

var jst = time.FixedZone("JST", 9*60*60)

var weekdayNames = map[string]time.Weekday{
	"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday,
	"wednesday": time.Wednesday, "thursday": time.Thursday,
	"friday": time.Friday, "saturday": time.Saturday,
}

// Config holds runtime configuration for usage computation.
type Config struct {
	LogDir            string
	SessionLimit      int // 5-hour block limit (0 = unknown)
	WeeklyLimit       int // weekly all-models limit (0 = unknown)
	WeeklySonnetLimit int // weekly Sonnet-only limit (0 = unknown)
	WeeklyResetDay    time.Weekday
	WeeklyResetHour   int // JST hour

	// DetectedTier is the rateLimitTier read from ~/.claude/.credentials.json.
	DetectedTier string
}

// ConfigFrom builds a service Config from an application config and auto-detected tier.
func ConfigFrom(c config.Config) Config {
	detectedTier, _ := plan.DetectTier()

	sessionLimit := c.PlanLimit
	if sessionLimit == 0 && detectedTier != "" {
		sessionLimit = plan.SessionLimitForTier(detectedTier)
	}
	weeklyLimit := c.WeeklyLimit
	if weeklyLimit == 0 && detectedTier != "" {
		weeklyLimit = plan.WeeklyLimitForTier(detectedTier)
	}
	weeklySonnetLimit := c.WeeklySonnetLimit
	if weeklySonnetLimit == 0 && detectedTier != "" {
		weeklySonnetLimit = plan.WeeklySonnetLimitForTier(detectedTier)
	}

	resetDay := time.Tuesday
	if d, ok := weekdayNames[strings.ToLower(c.WeeklyResetDay)]; ok {
		resetDay = d
	}

	return Config{
		LogDir:            c.LogDir,
		SessionLimit:      sessionLimit,
		WeeklyLimit:       weeklyLimit,
		WeeklySonnetLimit: weeklySonnetLimit,
		WeeklyResetDay:    resetDay,
		WeeklyResetHour:   c.WeeklyResetHour,
		DetectedTier:      detectedTier,
	}
}

// UsageResult holds session and weekly usage metrics.
type UsageResult struct {
	SessionTokens int
	SessionLimit  int
	SessionRatio  float64
	SessionEndsAt *time.Time
	ActiveBlock   *blocks.Block

	WeeklyTokens       int
	WeeklyLimit        int
	WeeklyRatio        float64
	WeeklySonnetTokens int
	WeeklySonnetLimit  int
	WeeklySonnetRatio  float64
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
		WeeklyResetsAt:    nextWeeklyReset(cfg.WeeklyResetDay, cfg.WeeklyResetHour),
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

	weekStart := lastWeeklyReset(cfg.WeeklyResetDay, cfg.WeeklyResetHour)
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

func lastWeeklyReset(day time.Weekday, hour int) time.Time {
	now := time.Now().In(jst)
	daysSince := (int(now.Weekday()) - int(day) + 7) % 7
	reset := time.Date(now.Year(), now.Month(), now.Day()-daysSince, hour, 0, 0, 0, jst)
	if now.Before(reset) {
		reset = reset.AddDate(0, 0, -7)
	}
	return reset
}

func nextWeeklyReset(day time.Weekday, hour int) time.Time {
	return lastWeeklyReset(day, hour).AddDate(0, 0, 7)
}

func totalTokens(e jsonl.UsageEntry) int {
	return e.InputTokens + e.OutputTokens + e.CacheCreationInputTokens + e.CacheReadInputTokens
}

func isSonnet(model string) bool {
	return strings.Contains(strings.ToLower(model), "sonnet")
}
