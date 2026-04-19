package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/rengotaku/claude-usage-tracker/internal/service"
)

type jsonOutput struct {
	TokensUsed    int     `json:"tokens_used"`
	PlanLimit     int     `json:"plan_limit"`
	UsageRatio    float64 `json:"usage_ratio"`
	BlockStartsAt string  `json:"block_starts_at,omitempty"`
	BlockEndsAt   string  `json:"block_ends_at,omitempty"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	jsonFlag := len(os.Args) > 1 && os.Args[1] == "--json"

	cfg := service.ConfigFromEnv()
	result, err := service.Compute(cfg)
	if err != nil {
		logger.Error("compute usage", "error", err)
		os.Exit(1)
	}

	if jsonFlag {
		out := jsonOutput{
			TokensUsed: result.TokensUsed,
			PlanLimit:  result.PlanLimit,
			UsageRatio: result.UsageRatio,
		}
		if result.ActiveBlock != nil {
			out.BlockStartsAt = result.ActiveBlock.StartTime.Format("2006-01-02T15:04:05Z")
			out.BlockEndsAt = result.ActiveBlock.EndTime.Format("2006-01-02T15:04:05Z")
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			logger.Error("encode json", "error", err)
			os.Exit(1)
		}
		return
	}

	pct := result.UsageRatio * 100
	usedM := float64(result.TokensUsed) / 1_000_000
	limitM := float64(result.PlanLimit) / 1_000_000

	if result.ActiveBlock == nil {
		fmt.Printf("%.1f%% (tokens: %.0fM / %.0fM, no active block)\n", pct, usedM, limitM)
		return
	}

	blockEnds := result.ActiveBlock.EndTime.Format("2006-01-02T15:04:05Z")
	fmt.Printf("%.1f%% (tokens: %.0fM / %.0fM, block ends at %s)\n", pct, usedM, limitM, blockEnds)
}
