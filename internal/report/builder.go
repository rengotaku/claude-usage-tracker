package report

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/blocks"
	"github.com/rengotaku/claude-usage-tracker/internal/numfmt"
	"github.com/rengotaku/claude-usage-tracker/internal/service"
)

var jst = time.FixedZone("JST", 9*60*60)

// Input holds all data needed to render a weekly usage report.
type Input struct {
	Now            time.Time
	OSType         string
	Usage          *service.UsageResult
	ModelBreakdown map[string]int // classified model name → total tokens
	RecentBlocks   []blocks.Block
	Monthly        *MonthlyData // nil if not in first week of month
}

// Build renders the Markdown report body.
func Build(in Input) string {
	nowStr := in.Now.In(jst).Format("2006-01-02 15:04 JST")
	var sb strings.Builder

	fmt.Fprintf(&sb, "## 週次レポート — %s\n\n", nowStr)
	fmt.Fprintf(&sb, "**実行ホスト**: %s\n\n", in.OSType)
	sb.WriteString(buildSession(in.Usage))
	sb.WriteString(buildWeekly(in.Usage, in.ModelBreakdown))
	sb.WriteString(buildBlocks(in.RecentBlocks))
	if in.Monthly != nil {
		sb.WriteString(buildMonthly(in.Monthly))
	}

	return strings.TrimRight(sb.String(), "\n") + "\n"
}

func buildSession(u *service.UsageResult) string {
	var sb strings.Builder
	sb.WriteString("### セッション (現在の5h ブロック)\n\n")

	usageLine := numfmt.Tokens(u.SessionTokens)
	if u.SessionLimit > 0 {
		usageLine = fmt.Sprintf("%s (%s / %s)", fmtPct(u.SessionRatio), numfmt.Tokens(u.SessionTokens), numfmt.Tokens(u.SessionLimit))
	}
	if u.SessionEndsAt != nil {
		usageLine += fmt.Sprintf(" — ブロック終了: %s", u.SessionEndsAt.In(jst).Format("2006-01-02T15:04:05+09:00"))
	}
	fmt.Fprintf(&sb, "- 使用率: %s\n\n", usageLine)

	b := u.SessionBreakdown
	sb.WriteString("**トークン内訳**\n\n")
	sb.WriteString("| input | output | cache_creation | cache_read |\n")
	sb.WriteString("|-------|--------|----------------|------------|\n")
	fmt.Fprintf(&sb, "| %s | %s | %s | %s |\n\n", numfmt.Tokens(b.Input), numfmt.Tokens(b.Output), numfmt.Tokens(b.CacheCreation), numfmt.Tokens(b.CacheRead))
	return sb.String()
}

func buildWeekly(u *service.UsageResult, modelBreakdown map[string]int) string {
	var sb strings.Builder
	sb.WriteString("### 週次使用量\n\n")

	allLine := numfmt.Tokens(u.WeeklyTokens)
	if u.WeeklyLimit > 0 {
		allLine = fmt.Sprintf("%s (%s / %s)", fmtPct(u.WeeklyRatio), numfmt.Tokens(u.WeeklyTokens), numfmt.Tokens(u.WeeklyLimit))
	}
	sonnetLine := numfmt.Tokens(u.WeeklySonnetTokens)
	if u.WeeklySonnetLimit > 0 {
		sonnetLine = fmt.Sprintf("%s (%s / %s)", fmtPct(u.WeeklySonnetRatio), numfmt.Tokens(u.WeeklySonnetTokens), numfmt.Tokens(u.WeeklySonnetLimit))
	}
	resetsAt := u.WeeklyResetsAt.In(jst).Format("2006-01-02T15:04:05+09:00")

	fmt.Fprintf(&sb, "- All: %s\n", allLine)
	fmt.Fprintf(&sb, "- Sonnet: %s\n", sonnetLine)
	fmt.Fprintf(&sb, "- リセット: %s\n\n", resetsAt)

	if len(modelBreakdown) > 0 {
		sb.WriteString("**週次モデル別内訳**\n\n")
		sb.WriteString("| モデル | トークン |\n")
		sb.WriteString("|--------|----------|\n")
		models := make([]string, 0, len(modelBreakdown))
		for m := range modelBreakdown {
			models = append(models, m)
		}
		sort.Slice(models, func(i, j int) bool {
			return modelBreakdown[models[i]] > modelBreakdown[models[j]]
		})
		for _, m := range models {
			fmt.Fprintf(&sb, "| %s | %s |\n", m, numfmt.Tokens(modelBreakdown[m]))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func buildBlocks(bs []blocks.Block) string {
	if len(bs) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("### 直近ブロック一覧 (直近7日)\n\n")
	sb.WriteString("| 開始 (JST) | 終了 (JST) | トークン |\n")
	sb.WriteString("|------------|------------|----------|\n")
	for _, b := range bs {
		end := b.EndTime.In(jst).Format("2006-01-02 15:04")
		if b.IsActive {
			end = "進行中"
		}
		fmt.Fprintf(&sb, "| %s | %s | %s |\n",
			b.StartTime.In(jst).Format("2006-01-02 15:04"),
			end,
			numfmt.Tokens(b.TotalTokens),
		)
	}
	sb.WriteString("\n")
	return sb.String()
}

func buildMonthly(m *MonthlyData) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "### 前月総括 (%s)\n\n", m.Label)
	fmt.Fprintf(&sb, "**月間トータル**: %s\n\n", numfmt.Tokens(m.TotalTokens))
	sb.WriteString("**モデル別内訳**\n\n")
	sb.WriteString("| モデル | トークン |\n")
	sb.WriteString("|--------|----------|\n")
	if len(m.ByModel) == 0 {
		sb.WriteString("| (データなし) | — |\n")
	} else {
		models := make([]string, 0, len(m.ByModel))
		for k := range m.ByModel {
			models = append(models, k)
		}
		sort.Slice(models, func(i, j int) bool {
			return m.ByModel[models[i]] > m.ByModel[models[j]]
		})
		for _, k := range models {
			fmt.Fprintf(&sb, "| %s | %s |\n", k, numfmt.Tokens(m.ByModel[k]))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func fmtPct(ratio float64) string {
	return fmt.Sprintf("%.0f%%", ratio*100)
}
