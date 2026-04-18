package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/jsonl"
)

func main() {
	dir := os.ExpandEnv("$HOME/.claude/projects")
	start := time.Now()
	entries, err := jsonl.Parse(dir)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("エントリ数: %d\n", len(entries))
	fmt.Printf("処理時間: %s\n", elapsed)
}
