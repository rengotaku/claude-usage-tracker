package server

import (
	"sort"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/repository"
)

var jst = time.FixedZone("JST", 9*60*60)

// BlockAgg represents a 5-hour block aggregation derived from snapshots.
type BlockAgg struct {
	Start  time.Time
	End    *time.Time
	Tokens int
	Ratio  float64
}

// DailyAgg represents one-day aggregation of session tokens.
type DailyAgg struct {
	Date   string // YYYY-MM-DD in JST
	Tokens int    // sum of block totals for that day
	Blocks int    // number of distinct 5h blocks
}

// AggregateBlocks groups snapshots by block_started_at and returns the
// block totals. Each block's Tokens is the MAX(tokens_used) within that
// group (since tokens_used is cumulative within an active block).
// Snapshots without a BlockEndedAt represent no-active-block placeholders
// (see cmd/snapshot) and are skipped.
func AggregateBlocks(snaps []repository.Snapshot) []BlockAgg {
	if len(snaps) == 0 {
		return nil
	}
	type key struct{ start int64 }
	byBlock := make(map[key]*BlockAgg)
	for _, s := range snaps {
		if s.BlockEndedAt == nil {
			continue
		}
		k := key{s.BlockStartedAt.Unix()}
		agg, ok := byBlock[k]
		if !ok {
			agg = &BlockAgg{Start: s.BlockStartedAt, End: s.BlockEndedAt}
			byBlock[k] = agg
		}
		if s.TokensUsed > agg.Tokens {
			agg.Tokens = s.TokensUsed
			agg.Ratio = s.UsageRatio
		}
	}
	result := make([]BlockAgg, 0, len(byBlock))
	for _, v := range byBlock {
		result = append(result, *v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Start.Before(result[j].Start) })
	return result
}

// AggregateDaily groups block aggregations into days by JST date.
func AggregateDaily(blocks []BlockAgg) []DailyAgg {
	if len(blocks) == 0 {
		return nil
	}
	byDate := make(map[string]*DailyAgg)
	for _, b := range blocks {
		d := b.Start.In(jst).Format("2006-01-02")
		agg, ok := byDate[d]
		if !ok {
			agg = &DailyAgg{Date: d}
			byDate[d] = agg
		}
		agg.Tokens += b.Tokens
		agg.Blocks++
	}
	result := make([]DailyAgg, 0, len(byDate))
	for _, v := range byDate {
		result = append(result, *v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Date < result[j].Date })
	return result
}

// maxSnapshotField returns the maximum value of field across snaps.
func maxSnapshotField(snaps []repository.Snapshot, field func(repository.Snapshot) int) int {
	var max int
	for _, s := range snaps {
		if v := field(s); v > max {
			max = v
		}
	}
	return max
}

// SumWeeklyTokens returns the latest weekly_tokens value observed in snaps
// (the tracker stores a running weekly counter; the max/last value represents
// the current weekly total for the range).
func SumWeeklyTokens(snaps []repository.Snapshot) int {
	return maxSnapshotField(snaps, func(s repository.Snapshot) int { return s.WeeklyTokens })
}
