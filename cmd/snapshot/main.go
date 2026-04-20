package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/repository"
	"github.com/rengotaku/claude-usage-tracker/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg := service.ConfigFromEnv()
	result, err := service.Compute(cfg)
	if err != nil {
		logger.Error("compute usage", "error", err)
		os.Exit(1)
	}

	dbPath := repository.DBPath()
	repo, err := repository.NewSnapshotRepository(context.Background(), dbPath)
	if err != nil {
		logger.Error("open repository", "path", dbPath, "error", err)
		os.Exit(1)
	}
	defer repo.Close()

	snap := repository.Snapshot{
		TakenAt:    time.Now().UTC(),
		TokensUsed: result.SessionTokens,
		UsageRatio: result.SessionRatio,
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
