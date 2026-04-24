package blocks

import (
	"sort"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/jsonl"
)

const blockDuration = 5 * time.Hour

// Block represents a 5-hour billing period.
type Block struct {
	StartTime   time.Time
	EndTime     time.Time
	IsActive    bool
	TotalTokens int
	EntryCount  int
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
				StartTime: start,
				EndTime:   start.Add(blockDuration),
			}
			blocks = append(blocks, b)
			current = &blocks[len(blocks)-1]
		}
		current.TotalTokens += totalTokens(e)
		current.EntryCount++
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


func totalTokens(e jsonl.UsageEntry) int {
	return e.InputTokens + e.OutputTokens + e.CacheCreationInputTokens + e.CacheReadInputTokens
}
