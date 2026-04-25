package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/config"
	"github.com/rengotaku/claude-usage-tracker/internal/repository"
	"github.com/rengotaku/claude-usage-tracker/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

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
	result, err := service.Compute(cfg)
	if err != nil {
		logger.Error("compute usage", "error", err)
		os.Exit(1)
	}

	repo, err := repository.NewSnapshotRepository(context.Background(), appCfg.DB)
	if err != nil {
		logger.Error("open repository", "path", appCfg.DB, "error", err)
		os.Exit(1)
	}
	defer repo.Close()

	snap := repository.Snapshot{
		TakenAt:            time.Now().UTC(),
		TokensUsed:         result.SessionTokens,
		UsageRatio:         result.SessionRatio,
		WeeklyTokens:       result.WeeklyTokens,
		WeeklySonnetTokens: result.WeeklySonnetTokens,
	}
	if result.ActiveBlock != nil {
		snap.BlockStartedAt = result.ActiveBlock.StartTime
		endTime := result.ActiveBlock.EndTime
		snap.BlockEndedAt = &endTime
	} else {
		snap.BlockStartedAt = time.Now().UTC()
	}

	if err := repo.Save(context.Background(), snap); err != nil {
		logger.Error("save snapshot", "error", err)
		os.Exit(1)
	}

	logger.Info("snapshot saved",
		"session_tokens", result.SessionTokens,
		"session_ratio", result.SessionRatio,
		"weekly_tokens", result.WeeklyTokens,
	)
}
