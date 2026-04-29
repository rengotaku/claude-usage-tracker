package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/config"
	"github.com/rengotaku/claude-usage-tracker/internal/logging"
	"github.com/rengotaku/claude-usage-tracker/internal/repository"
	"github.com/rengotaku/claude-usage-tracker/internal/server"
	"github.com/rengotaku/claude-usage-tracker/internal/service"
)

func main() {
	logger := logging.NewDefault()

	appCfg, svcCfg, err := service.LoadAndValidateConfig(config.DefaultPath())
	if err != nil {
		logger.Error("load or validate config", "error", err)
		os.Exit(1)
	}

	repo, err := repository.NewSnapshotRepository(context.Background(), appCfg.DB)
	if err != nil {
		logger.Error("open repository", "path", appCfg.DB, "error", err)
		os.Exit(1)
	}
	defer repo.Close()

	cfg := server.Config{
		SessionLimit:      svcCfg.SessionLimit,
		WeeklyLimit:       svcCfg.WeeklyLimit,
		WeeklySonnetLimit: svcCfg.WeeklySonnetLimit,
	}
	h := server.NewHandler(repo, cfg)

	port := appCfg.Port
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           h.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("server listening", "addr", srv.Addr, "db", appCfg.DB)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("serve", "error", err)
		os.Exit(1)
	}
}
