package server_test

import (
	"testing"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/repository"
	"github.com/rengotaku/claude-usage-tracker/internal/server"
)

func makeSnap(weeklyTokens, weeklySonnet int) repository.Snapshot {
	return repository.Snapshot{
		TakenAt:            time.Now(),
		BlockStartedAt:     time.Now(),
		WeeklyTokens:       weeklyTokens,
		WeeklySonnetTokens: weeklySonnet,
	}
}

func TestSumWeeklyTokens(t *testing.T) {
	tests := []struct {
		name  string
		snaps []repository.Snapshot
		want  int
	}{
		{name: "empty", snaps: nil, want: 0},
		{name: "single", snaps: []repository.Snapshot{makeSnap(100, 0)}, want: 100},
		{name: "returns max not last", snaps: []repository.Snapshot{makeSnap(300, 0), makeSnap(100, 0), makeSnap(200, 0)}, want: 300},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := server.SumWeeklyTokens(tc.snaps); got != tc.want {
				t.Errorf("SumWeeklyTokens() = %d, want %d", got, tc.want)
			}
		})
	}
}
