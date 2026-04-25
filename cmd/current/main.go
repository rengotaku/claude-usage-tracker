package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/config"
	"github.com/rengotaku/claude-usage-tracker/internal/service"
)

var jst = time.FixedZone("JST", 9*60*60)

type jsonOutput struct {
	Session struct {
		TokensUsed int     `json:"tokens_used"`
		Limit      int     `json:"limit,omitempty"`
		Ratio      float64 `json:"ratio,omitempty"`
		EndsAt     string  `json:"ends_at,omitempty"`
	} `json:"session"`
	Weekly struct {
		TokensUsed   int     `json:"tokens_used"`
		Limit        int     `json:"limit,omitempty"`
		Ratio        float64 `json:"ratio,omitempty"`
		SonnetTokens int     `json:"sonnet_tokens_used"`
		SonnetLimit  int     `json:"sonnet_limit,omitempty"`
		SonnetRatio  float64 `json:"sonnet_ratio,omitempty"`
		ResetsAt     string  `json:"resets_at"`
	} `json:"weekly"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	jsonFlag := len(os.Args) > 1 && os.Args[1] == "--json"

	appCfg, err := config.Load(config.DefaultPath())
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	cfg := service.ConfigFrom(appCfg)
	if err := service.ValidateConfig(cfg); err != nil {
		logger.Error("invalid config", "error", err)
		os.Exit(1)
	}
	logPlanDetection(logger, cfg)
	result, err := service.Compute(cfg)
	if err != nil {
		logger.Error("compute usage", "error", err)
		os.Exit(1)
	}

	if jsonFlag {
		out := jsonOutput{}
		out.Session.TokensUsed = result.SessionTokens
		out.Session.Limit = result.SessionLimit
		out.Session.Ratio = result.SessionRatio
		if result.SessionEndsAt != nil {
			out.Session.EndsAt = result.SessionEndsAt.In(jst).Format("2006-01-02T15:04:05+09:00")
		}
		out.Weekly.TokensUsed = result.WeeklyTokens
		out.Weekly.Limit = result.WeeklyLimit
		out.Weekly.Ratio = result.WeeklyRatio
		out.Weekly.SonnetTokens = result.WeeklySonnetTokens
		out.Weekly.SonnetLimit = result.WeeklySonnetLimit
		out.Weekly.SonnetRatio = result.WeeklySonnetRatio
		out.Weekly.ResetsAt = result.WeeklyResetsAt.In(jst).Format("2006-01-02T15:04:05+09:00")

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			logger.Error("encode json", "error", err)
			os.Exit(1)
		}
		return
	}

	resetsAt := result.WeeklyResetsAt.In(jst).Format("Jan 2, 3pm")
	fmt.Printf("Current session   %s\n", sessionLine(result))
	fmt.Printf("Weekly (All)      %s\n", weeklyLine(result.WeeklyTokens, result.WeeklyLimit, result.WeeklyRatio, resetsAt))
	fmt.Printf("Weekly (Sonnet)   %s\n", weeklyLine(result.WeeklySonnetTokens, result.WeeklySonnetLimit, result.WeeklySonnetRatio, resetsAt))
}

func sessionLine(r *service.UsageResult) string {
	endsAt := ""
	if r.SessionEndsAt != nil {
		endsAt = ", resets " + r.SessionEndsAt.In(jst).Format("3pm (Asia/Tokyo)")
	}
	if r.SessionLimit > 0 {
		pct := r.SessionRatio * 100
		return fmt.Sprintf("%.0f%% used (%s / %s%s)", pct, formatM(r.SessionTokens), formatM(r.SessionLimit), endsAt)
	}
	return fmt.Sprintf("%s used%s", formatM(r.SessionTokens), endsAt)
}

func weeklyLine(tokens, limit int, ratio float64, resetsAt string) string {
	if limit > 0 {
		return fmt.Sprintf("%.0f%% used (%s / %s, resets %s (Asia/Tokyo))", ratio*100, formatM(tokens), formatM(limit), resetsAt)
	}
	return fmt.Sprintf("%s used (resets %s (Asia/Tokyo))", formatM(tokens), resetsAt)
}

func logPlanDetection(logger *slog.Logger, cfg service.Config) {
	if cfg.DetectedTier == "" {
		return
	}
	logger.Info("plan detected",
		"tier", cfg.DetectedTier,
		"session_limit", cfg.SessionLimit,
	)
}

func formatM(n int) string {
	m := float64(n) / 1_000_000
	if m < 1 {
		return fmt.Sprintf("%.0fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%.0fM", m)
}
