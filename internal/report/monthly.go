package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/jsonl"
)

// MonthlyData holds aggregated stats for a previous month.
type MonthlyData struct {
	Label       string
	TotalTokens int
	ByModel     map[string]int
}

// ComputeMonthly returns monthly summary for the previous month if now is within the first 7 days
// of the month; otherwise returns nil.
func ComputeMonthly(entries []jsonl.UsageEntry, now time.Time) *MonthlyData {
	if now.Day() > 7 {
		return nil
	}

	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	prevEnd := firstOfMonth
	var prevStart time.Time
	if firstOfMonth.Month() == time.January {
		prevStart = time.Date(firstOfMonth.Year()-1, time.December, 1, 0, 0, 0, 0, time.UTC)
	} else {
		prevStart = time.Date(firstOfMonth.Year(), firstOfMonth.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	}

	label := fmt.Sprintf("%d-%02d", prevStart.Year(), prevStart.Month())
	byModel := map[string]int{}
	total := 0

	for _, e := range entries {
		ts := e.Timestamp.UTC()
		if ts.Before(prevStart) || !ts.Before(prevEnd) {
			continue
		}
		t := e.TotalTokens()
		total += t
		byModel[ClassifyModel(e.Model)] += t
	}

	return &MonthlyData{
		Label:       label,
		TotalTokens: total,
		ByModel:     byModel,
	}
}

// ClassifyModel maps a raw model identifier to a display category.
func ClassifyModel(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "sonnet"):
		return "Sonnet"
	case strings.Contains(m, "opus"):
		return "Opus"
	case strings.Contains(m, "haiku"):
		return "Haiku"
	default:
		return "Other"
	}
}