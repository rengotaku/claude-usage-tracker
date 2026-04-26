package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rengotaku/claude-usage-tracker/internal/blocks"
)

const timeLayout = time.RFC3339

// Snapshot represents a point-in-time record of token usage within a block.
type Snapshot struct {
	TakenAt            time.Time
	BlockStartedAt     time.Time
	BlockEndedAt       *time.Time
	TokensUsed         int
	Tokens             blocks.TokenBreakdown
	UsageRatio         float64
	WeeklyTokens       int
	WeeklySonnetTokens int
}

// SnapshotRepository persists Snapshot records to SQLite.
type SnapshotRepository struct {
	db *sql.DB
}


// NewSnapshotRepository opens (or creates) the SQLite database at dbPath and
// runs idempotent migrations. Pass ":memory:" for in-process testing.
func NewSnapshotRepository(ctx context.Context, dbPath string) (*SnapshotRepository, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	r := &SnapshotRepository{db: db}
	if err := r.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return r, nil
}

func (r *SnapshotRepository) migrate(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS snapshots (
			taken_at              TEXT PRIMARY KEY,
			block_started_at      TEXT NOT NULL,
			block_ended_at        TEXT,
			tokens_used           INTEGER NOT NULL,
			input_tokens          INTEGER NOT NULL DEFAULT 0,
			output_tokens         INTEGER NOT NULL DEFAULT 0,
			cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens     INTEGER NOT NULL DEFAULT 0,
			usage_ratio           REAL    NOT NULL,
			weekly_tokens         INTEGER NOT NULL DEFAULT 0,
			weekly_sonnet_tokens  INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	// Add columns to existing tables (idempotent).
	for _, col := range []string{
		"ALTER TABLE snapshots ADD COLUMN weekly_tokens         INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE snapshots ADD COLUMN weekly_sonnet_tokens  INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE snapshots ADD COLUMN input_tokens          INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE snapshots ADD COLUMN output_tokens         INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE snapshots ADD COLUMN cache_creation_tokens INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE snapshots ADD COLUMN cache_read_tokens     INTEGER NOT NULL DEFAULT 0",
	} {
		if _, err := r.db.ExecContext(ctx, col); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("alter table: %w", err)
			}
		}
	}
	return nil
}

// Save inserts or replaces a Snapshot (upsert by taken_at).
func (r *SnapshotRepository) Save(ctx context.Context, s Snapshot) error {
	var endedAt *string
	if s.BlockEndedAt != nil {
		v := s.BlockEndedAt.UTC().Truncate(time.Second).Format(timeLayout)
		endedAt = &v
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO snapshots
			(taken_at, block_started_at, block_ended_at, tokens_used, input_tokens, output_tokens,
			 cache_creation_tokens, cache_read_tokens, usage_ratio, weekly_tokens, weekly_sonnet_tokens)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.TakenAt.UTC().Truncate(time.Second).Format(timeLayout),
		s.BlockStartedAt.UTC().Truncate(time.Second).Format(timeLayout),
		endedAt,
		s.TokensUsed,
		s.Tokens.Input,
		s.Tokens.Output,
		s.Tokens.CacheCreation,
		s.Tokens.CacheRead,
		s.UsageRatio,
		s.WeeklyTokens,
		s.WeeklySonnetTokens,
	)
	if err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}
	return nil
}

// Latest returns the most recently taken Snapshot, or nil if none exists.
func (r *SnapshotRepository) Latest(ctx context.Context) (*Snapshot, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT taken_at, block_started_at, block_ended_at, tokens_used,
		        input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
		        usage_ratio, weekly_tokens, weekly_sonnet_tokens
		 FROM snapshots ORDER BY taken_at DESC LIMIT 1`)
	s, err := scanRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

// ListBetween returns Snapshots with taken_at in [from, to], ordered ascending.
func (r *SnapshotRepository) ListBetween(ctx context.Context, from, to time.Time) ([]Snapshot, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT taken_at, block_started_at, block_ended_at, tokens_used,
		        input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
		        usage_ratio, weekly_tokens, weekly_sonnet_tokens
		 FROM snapshots
		 WHERE taken_at >= ? AND taken_at <= ?
		 ORDER BY taken_at`,
		from.UTC().Format(timeLayout),
		to.UTC().Format(timeLayout),
	)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	defer rows.Close()

	var result []Snapshot
	for rows.Next() {
		s, err := scanRows(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *s)
	}
	return result, rows.Err()
}

// Close releases the underlying database connection.
func (r *SnapshotRepository) Close() error {
	return r.db.Close()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRow(s *sql.Row) (*Snapshot, error) {
	return scan(s)
}

func scanRows(s *sql.Rows) (*Snapshot, error) {
	return scan(s)
}

func scan(s scanner) (*Snapshot, error) {
	var (
		takenAt            string
		blockStartedAt     string
		blockEndedAt       *string
		tokensUsed         int
		inputTokens        int
		outputTokens       int
		cacheCreation      int
		cacheRead          int
		usageRatio         float64
		weeklyTokens       int
		weeklySonnetTokens int
	)
	if err := s.Scan(&takenAt, &blockStartedAt, &blockEndedAt, &tokensUsed,
		&inputTokens, &outputTokens, &cacheCreation, &cacheRead,
		&usageRatio, &weeklyTokens, &weeklySonnetTokens); err != nil {
		return nil, err
	}

	ta, err := time.Parse(timeLayout, takenAt)
	if err != nil {
		return nil, fmt.Errorf("parse taken_at: %w", err)
	}
	bs, err := time.Parse(timeLayout, blockStartedAt)
	if err != nil {
		return nil, fmt.Errorf("parse block_started_at: %w", err)
	}

	snap := &Snapshot{
		TakenAt:        ta.UTC(),
		BlockStartedAt: bs.UTC(),
		TokensUsed:     tokensUsed,
		Tokens: blocks.TokenBreakdown{
			Input:         inputTokens,
			Output:        outputTokens,
			CacheCreation: cacheCreation,
			CacheRead:     cacheRead,
		},
		UsageRatio:         usageRatio,
		WeeklyTokens:       weeklyTokens,
		WeeklySonnetTokens: weeklySonnetTokens,
	}
	if blockEndedAt != nil {
		be, err := time.Parse(timeLayout, *blockEndedAt)
		if err != nil {
			return nil, fmt.Errorf("parse block_ended_at: %w", err)
		}
		t := be.UTC()
		snap.BlockEndedAt = &t
	}
	return snap, nil
}
