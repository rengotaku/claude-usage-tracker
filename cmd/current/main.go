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
	"github.com/rengotaku/claude-usage-tracker/internal/service"
)

var jst = time.FixedZone("JST", 9*60*60)

type tokenBreakdownJSON struct {
	Input         int `json:"input"`
	Output        int `json:"output"`
	CacheCreation int `json:"cache_creation"`
	CacheRead     int `json:"cache_read"`
}

type jsonOutput struct {
	Session struct {
		TokensUsed int                `json:"tokens_used"`
		Breakdown  tokenBreakdownJSON `json:"breakdown"`
		Limit      int                `json:"limit,omitempty"`
		Ratio      float64            `json:"ratio,omitempty"`
		EndsAt     string             `json:"ends_at,omitempty"`
	} `json:"session"`
	Weekly struct {
		TokensUsed     int                            `json:"tokens_used"`
		Limit          int                            `json:"limit,omitempty"`
		Ratio          float64                        `json:"ratio,omitempty"`
		SonnetTokens   int                            `json:"sonnet_tokens_used"`
		SonnetLimit    int                            `json:"sonnet_limit,omitempty"`
		SonnetRatio    float64                        `json:"sonnet_ratio,omitempty"`
		ResetsAt       string                         `json:"resets_at"`
		ModelBreakdown map[string]tokenBreakdownJSON  `json:"model_breakdown,omitempty"`
	} `json:"weekly"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

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

	appCfg, cfg, err := service.LoadAndValidateConfig()
	if err != nil {
		logger.Error("config", "error", err)
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
		out.Session.Breakdown = tokenBreakdownJSON{
			Input:         result.SessionBreakdown.Input,
			Output:        result.SessionBreakdown.Output,
			CacheCreation: result.SessionBreakdown.CacheCreation,
			CacheRead:     result.SessionBreakdown.CacheRead,
		}
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
		if len(result.WeeklyModelBreakdown) > 0 {
			out.Weekly.ModelBreakdown = make(map[string]tokenBreakdownJSON, len(result.WeeklyModelBreakdown))
			for model, bd := range result.WeeklyModelBreakdown {
				out.Weekly.ModelBreakdown[model] = tokenBreakdownJSON{
					Input:         bd.Input,
					Output:        bd.Output,
					CacheCreation: bd.CacheCreation,
					CacheRead:     bd.CacheRead,
				}
			}
		}

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
	b := result.SessionBreakdown
	fmt.Printf("  input %-8s  output %-8s  cache_creation %-8s  cache_read %s\n",
		formatM(b.Input), formatM(b.Output), formatM(b.CacheCreation), formatM(b.CacheRead))
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
			fmt.Printf("  %-38s %s\n", m, formatM(bd.Total()))
		}
	}
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
