package blocks

import (
	"sort"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/jsonl"
)

const blockDuration = 5 * time.Hour

// TokenBreakdown holds per-type token counts for a block.
type TokenBreakdown struct {
	Input         int
	Output        int
	CacheCreation int
	CacheRead     int
}

// Total returns the sum of all token types.
func (t TokenBreakdown) Total() int {
	return t.Input + t.Output + t.CacheCreation + t.CacheRead
}

// FromEntry constructs a TokenBreakdown from a single usage entry.
func FromEntry(e jsonl.UsageEntry) TokenBreakdown {
	return TokenBreakdown{
		Input:         e.InputTokens,
		Output:        e.OutputTokens,
		CacheCreation: e.CacheCreationInputTokens,
		CacheRead:     e.CacheReadInputTokens,
	}
}

// Add returns the sum of two breakdowns. The receiver is not mutated.
func (t TokenBreakdown) Add(other TokenBreakdown) TokenBreakdown {
	return TokenBreakdown{
		Input:         t.Input + other.Input,
		Output:        t.Output + other.Output,
		CacheCreation: t.CacheCreation + other.CacheCreation,
		CacheRead:     t.CacheRead + other.CacheRead,
	}
}

// Block represents a 5-hour billing period.
type Block struct {
	StartTime      time.Time
	EndTime        time.Time
	IsActive       bool
	TotalTokens    int
	Tokens         TokenBreakdown
	EntryCount     int
	ModelBreakdown map[string]TokenBreakdown // per-model token breakdown; empty string keys excluded
}

// Build converts a slice of UsageEntry into 5-hour blocks sorted by StartTime.
func Build(entries []jsonl.UsageEntry) []Block {
	if len(entries) == 0 {
		return nil
	}

	sorted := make([]jsonl.UsageEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	var blocks []Block
	var current *Block

	for _, e := range sorted {
		if current == nil || !e.Timestamp.Before(current.EndTime) {
			start := e.Timestamp.UTC().Truncate(time.Second)
			b := Block{
				StartTime:      start,
				EndTime:        start.Add(blockDuration),
				ModelBreakdown: make(map[string]TokenBreakdown),
			}
			blocks = append(blocks, b)
			current = &blocks[len(blocks)-1]
		}
		bd := FromEntry(e)
		current.TotalTokens += bd.Total()
		current.Tokens = current.Tokens.Add(bd)
		current.EntryCount++
		if e.Model != "" {
			current.ModelBreakdown[e.Model] = current.ModelBreakdown[e.Model].Add(bd)
		}
	}

	now := time.Now().UTC()
	for i := range blocks {
		if !now.Before(blocks[i].StartTime) && now.Before(blocks[i].EndTime) {
			blocks[i].IsActive = true
		}
	}

	return blocks
}

// ActiveBlock returns the currently active block, or nil if none is active.
func ActiveBlock(blocks []Block) *Block {
	for i := range blocks {
		if blocks[i].IsActive {
			return &blocks[i]
		}
	}
	return nil
}
