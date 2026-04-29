package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/blocks"
	"github.com/rengotaku/claude-usage-tracker/internal/config"
	"github.com/rengotaku/claude-usage-tracker/internal/jsonl"
	"github.com/rengotaku/claude-usage-tracker/internal/report"
	"github.com/rengotaku/claude-usage-tracker/internal/service"
	"github.com/rengotaku/claude-usage-tracker/internal/timezone"
)

const lastReportFile = ".local/share/claude-usage-tracker/last-usage-report.txt"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	dryRun := false
	force := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--force":
			force = true
		}
	}

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

	usage, err := service.Compute(cfg)
	if err != nil {
		logger.Error("compute usage", "error", err)
		os.Exit(1)
	}

	entries, err := jsonl.Parse(appCfg.LogDir)
	if err != nil {
		logger.Warn("parse jsonl", "error", err)
	}

	modelBreakdown := computeModelBreakdown(entries, usage.WeeklyStartsAt)

	recentBlocks := buildRecentBlocks(entries)

	now := time.Now().In(timezone.JST)
	monthly := report.ComputeMonthly(entries, now)

	osType := "linux"
	if runtime.GOOS == "darwin" {
		osType = "mac"
	}

	body := report.Build(report.Input{
		Now:            now,
		OSType:         osType,
		Usage:          usage,
		ModelBreakdown: modelBreakdown,
		RecentBlocks:   recentBlocks,
		Monthly:        monthly,
	})

	lastReportPath := filepath.Join(os.Getenv("HOME"), lastReportFile)

	if !force && !dryRun {
		if prev, err := os.ReadFile(lastReportPath); err == nil && string(prev) == body {
			fmt.Println("Report unchanged since last post. Use --force to override.")
			return
		}
	}

	if dryRun {
		fmt.Print(body)
		fmt.Println("[dry-run] コメント投稿をスキップしました")
		return
	}

	discussionID, err := report.GetDiscussionID(report.RepoOwner, report.RepoName, report.DiscussionNum)
	if err != nil {
		logger.Error("get discussion id", "error", err)
		os.Exit(1)
	}

	url, err := report.PostComment(discussionID, body)
	if err != nil {
		logger.Error("post comment", "error", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(lastReportPath), 0o755); err != nil {
		logger.Warn("create report dir", "error", err)
	} else if err := os.WriteFile(lastReportPath, []byte(body), 0o644); err != nil {
		logger.Warn("write last report", "error", err)
	}

	fmt.Printf("Posted: %s\n", url)
}

func computeModelBreakdown(entries []jsonl.UsageEntry, weekStart time.Time) map[string]int {
	byModel := map[string]int{}
	for _, e := range entries {
		if e.Timestamp.Before(weekStart) {
			continue
		}
		byModel[report.ClassifyModel(e.Model)] += e.TotalTokens()
	}
	return byModel
}

func buildRecentBlocks(entries []jsonl.UsageEntry) []blocks.Block {
	bs := blocks.Build(entries)
	cutoff := time.Now().AddDate(0, 0, -7)
	var result []blocks.Block
	for i := len(bs) - 1; i >= 0; i-- {
		b := bs[i]
		if b.StartTime.Before(cutoff) || b.TotalTokens == 0 {
			continue
		}
		result = append(result, b)
	}
	return result
}
