package jsonl

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type UsageEntry struct {
	MessageID                string
	Timestamp                time.Time
	Model                    string
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

type rawEntry struct {
	Type      string    `json:"type"`
	UUID      string    `json:"uuid"`
	Timestamp time.Time `json:"timestamp"`
	Message   *struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// Parse walks dir and returns all unique UsageEntry records found in .jsonl files.
func Parse(dir string) ([]UsageEntry, error) {
	seen := make(map[string]struct{})
	var entries []UsageEntry

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		fileEntries, fileErr := parseFile(path, seen)
		if fileErr != nil {
			log.Printf("warn: skipping file %s: %v", path, fileErr)
			return nil
		}
		entries = append(entries, fileEntries...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func parseFile(path string, seen map[string]struct{}) ([]UsageEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []UsageEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var raw rawEntry
		if err := json.Unmarshal(line, &raw); err != nil {
			log.Printf("warn: skipping malformed line in %s: %v", path, err)
			continue
		}
		if raw.Type != "assistant" || raw.Message == nil || raw.Message.Usage == nil {
			continue
		}
		messageID := raw.Message.ID
		if messageID == "" {
			messageID = raw.UUID
		}
		if _, dup := seen[messageID]; dup {
			continue
		}
		seen[messageID] = struct{}{}
		entries = append(entries, UsageEntry{
			MessageID:                messageID,
			Timestamp:                raw.Timestamp,
			Model:                    raw.Message.Model,
			InputTokens:              raw.Message.Usage.InputTokens,
			OutputTokens:             raw.Message.Usage.OutputTokens,
			CacheCreationInputTokens: raw.Message.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     raw.Message.Usage.CacheReadInputTokens,
		})
	}
	return entries, scanner.Err()
}
