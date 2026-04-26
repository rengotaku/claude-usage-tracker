package report_test

import (
	"strings"
	"testing"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/blocks"
	"github.com/rengotaku/claude-usage-tracker/internal/report"
	"github.com/rengotaku/claude-usage-tracker/internal/service"
)

var jst = time.FixedZone("JST", 9*60*60)

func fixedTime(year, month, day, hour int) time.Time {
	return time.Date(year, time.Month(month), day, hour, 0, 0, 0, jst)
}

func TestBuild_ContainsHeader(t *testing.T) {
	now := fixedTime(2026, 4, 27, 10)
	input := report.Input{
		Now:    now,
		OSType: "linux",
		Usage:  &service.UsageResult{},
	}
	got := report.Build(input)
	if !strings.Contains(got, "週次レポート") {
		t.Errorf("expected header '週次レポート', got: %s", got)
	}
	if !strings.Contains(got, "linux") {
		t.Errorf("expected OSType 'linux' in report, got: %s", got)
	}
	if !strings.Contains(got, "2026-04-27") {
		t.Errorf("expected date '2026-04-27' in report, got: %s", got)
	}
}

func TestBuild_SessionSection(t *testing.T) {
	now := fixedTime(2026, 4, 27, 10)
	sessionEnd := fixedTime(2026, 4, 27, 12)
	input := report.Input{
		Now:    now,
		OSType: "linux",
		Usage: &service.UsageResult{
			SessionTokens: 1_500_000,
			SessionLimit:  88_000,
			SessionRatio:  0.75,
			SessionEndsAt: &sessionEnd,
			SessionBreakdown: blocks.TokenBreakdown{
				Input:         500_000,
				Output:        300_000,
				CacheCreation: 400_000,
				CacheRead:     300_000,
			},
		},
	}
	got := report.Build(input)
	if !strings.Contains(got, "セッション") {
		t.Errorf("expected session section, got: %s", got)
	}
	if !strings.Contains(got, "75%") {
		t.Errorf("expected 75%%, got: %s", got)
	}
}

func TestBuild_WeeklySection(t *testing.T) {
	now := fixedTime(2026, 4, 27, 10)
	resetsAt := fixedTime(2026, 4, 28, 9)
	input := report.Input{
		Now:    now,
		OSType: "linux",
		Usage: &service.UsageResult{
			WeeklyTokens:       310_000_000,
			WeeklyLimit:        1_000_000_000,
			WeeklyRatio:        0.31,
			WeeklySonnetTokens: 280_000_000,
			WeeklySonnetLimit:  800_000_000,
			WeeklySonnetRatio:  0.35,
			WeeklyResetsAt:     resetsAt,
		},
		ModelBreakdown: map[string]int{
			"Sonnet": 280_000_000,
			"Opus":   30_000_000,
		},
	}
	got := report.Build(input)
	if !strings.Contains(got, "週次使用量") {
		t.Errorf("expected weekly section, got: %s", got)
	}
	if !strings.Contains(got, "31%") {
		t.Errorf("expected 31%%, got: %s", got)
	}
	if !strings.Contains(got, "Sonnet") {
		t.Errorf("expected Sonnet in model breakdown, got: %s", got)
	}
}

func TestBuild_RecentBlocks(t *testing.T) {
	now := fixedTime(2026, 4, 27, 10)
	blockStart := fixedTime(2026, 4, 26, 8)
	blockEnd := fixedTime(2026, 4, 26, 13)
	input := report.Input{
		Now:    now,
		OSType: "linux",
		Usage:  &service.UsageResult{},
		RecentBlocks: []blocks.Block{
			{
				StartTime:   blockStart,
				EndTime:     blockEnd,
				TotalTokens: 2_000_000,
			},
		},
	}
	got := report.Build(input)
	if !strings.Contains(got, "直近ブロック") {
		t.Errorf("expected recent blocks section, got: %s", got)
	}
	if !strings.Contains(got, "2M") {
		t.Errorf("expected 2M tokens in blocks, got: %s", got)
	}
}

func TestBuild_NoRecentBlocks_SkipsSection(t *testing.T) {
	now := fixedTime(2026, 4, 27, 10)
	input := report.Input{
		Now:    now,
		OSType: "mac",
		Usage:  &service.UsageResult{},
	}
	got := report.Build(input)
	if strings.Contains(got, "直近ブロック") {
		t.Errorf("expected no recent blocks section when empty, got: %s", got)
	}
}

func TestBuild_MonthlySummary(t *testing.T) {
	now := fixedTime(2026, 4, 27, 10)
	input := report.Input{
		Now:    now,
		OSType: "linux",
		Usage:  &service.UsageResult{},
		Monthly: &report.MonthlyData{
			Label:       "2026-03",
			TotalTokens: 5_000_000,
			ByModel: map[string]int{
				"Sonnet": 4_000_000,
				"Haiku":  1_000_000,
			},
		},
	}
	got := report.Build(input)
	if !strings.Contains(got, "前月総括") {
		t.Errorf("expected monthly section, got: %s", got)
	}
	if !strings.Contains(got, "2026-03") {
		t.Errorf("expected label 2026-03, got: %s", got)
	}
	if !strings.Contains(got, "5M") {
		t.Errorf("expected 5M total tokens, got: %s", got)
	}
}

func TestBuild_NoMonthly_SkipsSection(t *testing.T) {
	now := fixedTime(2026, 4, 27, 10)
	input := report.Input{
		Now:    now,
		OSType: "linux",
		Usage:  &service.UsageResult{},
	}
	got := report.Build(input)
	if strings.Contains(got, "前月総括") {
		t.Errorf("expected no monthly section when nil, got: %s", got)
	}
}
