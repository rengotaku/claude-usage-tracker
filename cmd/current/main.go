package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/cache"
	"github.com/rengotaku/claude-usage-tracker/internal/config"
	"github.com/rengotaku/claude-usage-tracker/internal/logging"
	"github.com/rengotaku/claude-usage-tracker/internal/numfmt"
	"github.com/rengotaku/claude-usage-tracker/internal/service"
	"github.com/rengotaku/claude-usage-tracker/internal/tz"
	"github.com/rengotaku/claude-usage-tracker/internal/usagejson"
)


type jsonOutput struct {
	Session struct {
		TokensUsed int                      `json:"tokens_used"`
		Breakdown  usagejson.TokenBreakdown `json:"breakdown"`
		Limit      int                      `json:"limit,omitempty"`
		Ratio      float64                  `json:"ratio,omitempty"`
		EndsAt     string                   `json:"ends_at,omitempty"`
	} `json:"session"`
	Weekly struct {
		TokensUsed     int                                 `json:"tokens_used"`
		Limit          int                                 `json:"limit,omitempty"`
		Ratio          float64                             `json:"ratio,omitempty"`
		SonnetTokens   int                                 `json:"sonnet_tokens_used"`
		SonnetLimit    int                                 `json:"sonnet_limit,omitempty"`
		SonnetRatio    float64                             `json:"sonnet_ratio,omitempty"`
		ResetsAt       string                              `json:"resets_at"`
		ModelBreakdown map[string]usagejson.TokenBreakdown `json:"model_breakdown,omitempty"`
	} `json:"weekly"`
}

func main() {
	logger := logging.NewDefault()

	noCache := false
	jsonFlag := false
	modelBreakdown := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--json":
			jsonFlag = true
		case "--no-cache":
			noCache = true
		case "--model-breakdown":
			modelBreakdown = true
		}
	}

	appCfg, cfg, err := service.LoadAndValidateConfig(config.DefaultPath())
	if err != nil {
		logger.Error("load or validate config", "error", err)
		os.Exit(1)
	}

	cachePath := filepath.Join(filepath.Dir(appCfg.DB), "current-cache.json")
	ttl := time.Duration(appCfg.CacheTTL) * time.Second

	var result *service.UsageResult

	if !noCache {
		result, err = cache.Load(cachePath, ttl)
		if err != nil {
			logger.Warn("cache load failed", "error", err)
		}
	}

	if result == nil {
		logPlanDetection(logger, cfg)
		result, err = service.Compute(cfg)
		if err != nil {
			logger.Error("compute usage", "error", err)
			os.Exit(1)
		}
		if err := cache.Save(cachePath, result); err != nil {
			logger.Warn("cache save failed", "error", err)
		}
	}

	if jsonFlag {
		out := jsonOutput{}
		out.Session.TokensUsed = result.SessionTokens
		out.Session.Breakdown = usagejson.FromBlocks(result.SessionBreakdown)
		out.Session.Limit = result.SessionLimit
		out.Session.Ratio = result.SessionRatio
		if result.SessionEndsAt != nil {
			out.Session.EndsAt = result.SessionEndsAt.In(tz.JST).Format(time.RFC3339)
		}
		out.Weekly.TokensUsed = result.WeeklyTokens
		out.Weekly.Limit = result.WeeklyLimit
		out.Weekly.Ratio = result.WeeklyRatio
		out.Weekly.SonnetTokens = result.WeeklySonnetTokens
		out.Weekly.SonnetLimit = result.WeeklySonnetLimit
		out.Weekly.SonnetRatio = result.WeeklySonnetRatio
		out.Weekly.ResetsAt = result.WeeklyResetsAt.In(tz.JST).Format(time.RFC3339)
		out.Weekly.ModelBreakdown = usagejson.MapFromBlocks(result.WeeklyModelBreakdown)

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			logger.Error("encode json", "error", err)
			os.Exit(1)
		}
		return
	}

	resetsAt := result.WeeklyResetsAt.In(tz.JST).Format("Jan 2, 3pm")
	fmt.Printf("Current session   %s\n", sessionLine(result))
	b := result.SessionBreakdown
	fmt.Printf("  input %-8s  output %-8s  cache_creation %-8s  cache_read %s\n",
		numfmt.Tokens(b.Input), numfmt.Tokens(b.Output), numfmt.Tokens(b.CacheCreation), numfmt.Tokens(b.CacheRead))
	fmt.Printf("Weekly (All)      %s\n", weeklyLine(result.WeeklyTokens, result.WeeklyLimit, result.WeeklyRatio, resetsAt))
	fmt.Printf("Weekly (Sonnet)   %s\n", weeklyLine(result.WeeklySonnetTokens, result.WeeklySonnetLimit, result.WeeklySonnetRatio, resetsAt))

	if modelBreakdown && len(result.WeeklyModelBreakdown) > 0 {
		fmt.Println("Weekly by model:")
		models := make([]string, 0, len(result.WeeklyModelBreakdown))
		for m := range result.WeeklyModelBreakdown {
			models = append(models, m)
		}
		sort.Strings(models)
		for _, m := range models {
			bd := result.WeeklyModelBreakdown[m]
			fmt.Printf("  %-38s %s\n", m, numfmt.Tokens(bd.Total()))
		}
	}
}

func sessionLine(r *service.UsageResult) string {
	endsAt := ""
	if r.SessionEndsAt != nil {
		endsAt = ", resets " + r.SessionEndsAt.In(tz.JST).Format("3pm (Asia/Tokyo)")
	}
	if r.SessionLimit > 0 {
		pct := r.SessionRatio * 100
		return fmt.Sprintf("%.0f%% used (%s / %s%s)", pct, numfmt.Tokens(r.SessionTokens), numfmt.Tokens(r.SessionLimit), endsAt)
	}
	return fmt.Sprintf("%s used%s", numfmt.Tokens(r.SessionTokens), endsAt)
}

func weeklyLine(tokens, limit int, ratio float64, resetsAt string) string {
	if limit > 0 {
		return fmt.Sprintf("%.0f%% used (%s / %s, resets %s (Asia/Tokyo))", ratio*100, numfmt.Tokens(tokens), numfmt.Tokens(limit), resetsAt)
	}
	return fmt.Sprintf("%s used (resets %s (Asia/Tokyo))", numfmt.Tokens(tokens), resetsAt)
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
