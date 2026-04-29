package logging

import (
	"log/slog"
	"os"
)

// NewDefault returns the standard slog.Logger used by all CLI binaries:
// JSON output written to stderr.
func NewDefault() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}
