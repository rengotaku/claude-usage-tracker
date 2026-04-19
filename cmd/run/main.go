package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/blocks"
	"github.com/rengotaku/claude-usage-tracker/internal/jsonl"
)

func main() {
	dir := os.ExpandEnv("$HOME/.claude/projects")

	start := time.Now()
	entries, err := jsonl.Parse(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	bs := blocks.Build(entries)
	elapsed := time.Since(start)

	fmt.Printf("エントリ数: %d\n", len(entries))
	fmt.Printf("ブロック数: %d\n", len(bs))
	fmt.Printf("処理時間: %s\n", elapsed)

	active := blocks.ActiveBlock(bs)
	if active != nil {
		fmt.Printf("\n--- アクティブブロック ---\n")
		fmt.Printf("開始: %s\n", active.StartTime.Format(time.RFC3339))
		fmt.Printf("終了: %s\n", active.EndTime.Format(time.RFC3339))
		fmt.Printf("トークン合計: %d\n", active.TotalTokens)
		fmt.Printf("エントリ数: %d\n", active.EntryCount)
	} else {
		fmt.Println("\nアクティブブロック: なし")
	}

	fmt.Printf("\n--- 直近3ブロック ---\n")
	start3 := len(bs) - 3
	if start3 < 0 {
		start3 = 0
	}
	for _, b := range bs[start3:] {
		status := ""
		if b.IsActive {
			status = " [ACTIVE]"
		}
		fmt.Printf("%s ~ %s | tokens=%d entries=%d%s\n",
			b.StartTime.Format("01/02 15:04"),
			b.EndTime.Format("01/02 15:04"),
			b.TotalTokens,
			b.EntryCount,
			status,
		)
	}
}
