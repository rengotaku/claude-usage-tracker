package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/repository"
	"github.com/rengotaku/claude-usage-tracker/internal/server"
)

const (
	defaultPort = "8080"
	envPort     = "CLAUDE_USAGE_TRACKER_PORT"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	port := os.Getenv(envPort)
	if port == "" {
		port = defaultPort
	}

	dbPath := repository.DBPath()
	repo, err := repository.NewSnapshotRepository(context.Background(), dbPath)
	if err != nil {
		logger.Error("open repository", "path", dbPath, "error", err)
		os.Exit(1)
	}
	defer repo.Close()

	cfg := server.Config{
		SessionLimit:      envInt("CLAUDE_USAGE_TRACKER_PLAN_LIMIT"),
		WeeklyLimit:       envInt("CLAUDE_USAGE_TRACKER_WEEKLY_LIMIT"),
		WeeklySonnetLimit: envInt("CLAUDE_USAGE_TRACKER_WEEKLY_SONNET_LIMIT"),
	}
	h := server.NewHandler(repo, cfg)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           h.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("server listening", "addr", srv.Addr, "db", dbPath)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("serve", "error", err)
		os.Exit(1)
	}
}

func envInt(key string) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 0
}
