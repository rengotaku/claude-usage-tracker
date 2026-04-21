package server

import (
	"context"
	"net/http"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/repository"
)

// SnapshotLister is the minimal repository interface the handlers depend on.
type SnapshotLister interface {
	ListBetween(ctx context.Context, from, to time.Time) ([]repository.Snapshot, error)
}

// Config holds server configuration.
type Config struct {
	SessionLimit      int // 5h block limit (0 = unknown)
	WeeklyLimit       int // weekly all-models limit (0 = unknown)
	WeeklySonnetLimit int // weekly Sonnet limit (0 = unknown)
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	repo SnapshotLister
	cfg  Config
}

// NewHandler returns a Handler bound to the given repository and config.
func NewHandler(repo SnapshotLister, cfg Config) *Handler {
	return &Handler{repo: repo, cfg: cfg}
}

// Routes returns an http.ServeMux with all /usage/* routes registered.
func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /usage/snapshots", h.Snapshots)
	mux.HandleFunc("GET /usage/blocks", h.Blocks)
	mux.HandleFunc("GET /usage/daily", h.Daily)
	mux.HandleFunc("GET /usage/summary", h.Summary)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}
