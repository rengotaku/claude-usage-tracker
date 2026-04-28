package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/blocks"
	"github.com/rengotaku/claude-usage-tracker/internal/repository"
)

// timeFormat for JSON output (JST).
const jsonTimeFormat = "2006-01-02T15:04:05-07:00"

type errorResponse struct {
	Error string `json:"error"`
}

// formatJST renders t in JST using jsonTimeFormat.
func formatJST(t time.Time) string {
	return t.In(jst).Format(jsonTimeFormat)
}

// newPeriod builds a periodDTO from a [from, to] pair.
func newPeriod(from, to time.Time) periodDTO {
	return periodDTO{From: formatJST(from), To: formatJST(to)}
}

// toTokenBreakdown converts a blocks.TokenBreakdown to its DTO form.
func toTokenBreakdown(b blocks.TokenBreakdown) tokenBreakdown {
	return tokenBreakdown{
		Input:         b.Input,
		Output:        b.Output,
		CacheCreation: b.CacheCreation,
		CacheRead:     b.CacheRead,
	}
}

// loadRange parses from/to query params and loads snapshots in that range.
// On failure it writes the appropriate error response and returns ok=false.
func (h *Handler) loadRange(w http.ResponseWriter, r *http.Request, defaultWindow time.Duration) (snaps []repository.Snapshot, from, to time.Time, ok bool) {
	from, to, err := parseRange(r, defaultWindow)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return nil, time.Time{}, time.Time{}, false
	}
	snaps, err = h.repo.ListBetween(r.Context(), from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return nil, time.Time{}, time.Time{}, false
	}
	return snaps, from, to, true
}

// parseRange parses from/to query params. Supports RFC3339 or YYYY-MM-DD.
// If either is missing, applies the provided default window ending at now.
func parseRange(r *http.Request, defaultWindow time.Duration) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	to := now
	from := now.Add(-defaultWindow)

	if v := r.URL.Query().Get("from"); v != "" {
		t, err := parseFlexibleTime(v, false)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid from: %w", err)
		}
		from = t
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := parseFlexibleTime(v, true)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid to: %w", err)
		}
		to = t
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("to must be >= from")
	}
	return from, to, nil
}

// parseFlexibleTime parses RFC3339 or YYYY-MM-DD. For date-only, endOfDay
// controls whether the time is 00:00:00 or 23:59:59 (JST-anchored).
func parseFlexibleTime(v string, endOfDay bool) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.ParseInLocation("2006-01-02", v, jst); err == nil {
		if endOfDay {
			t = t.Add(24*time.Hour - time.Second)
		}
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("must be RFC3339 or YYYY-MM-DD, got %q", v)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// --- /usage/snapshots ---

type tokenBreakdown struct {
	Input         int `json:"input"`
	Output        int `json:"output"`
	CacheCreation int `json:"cache_creation"`
	CacheRead     int `json:"cache_read"`
}

type snapshotItem struct {
	TakenAt              string                    `json:"taken_at"`
	BlockStartedAt       string                    `json:"block_started_at"`
	BlockEndedAt         string                    `json:"block_ended_at,omitempty"`
	SessionTokens        int                       `json:"session_tokens"`
	Tokens               tokenBreakdown            `json:"tokens"`
	SessionRatio         float64                   `json:"session_ratio"`
	WeeklyTokens         int                       `json:"weekly_tokens"`
	WeeklySonnetTokens   int                       `json:"weekly_sonnet_tokens"`
	WeeklyModelBreakdown map[string]tokenBreakdown `json:"weekly_model_breakdown,omitempty"`
}

type snapshotsResponse struct {
	Period    periodDTO      `json:"period"`
	Snapshots []snapshotItem `json:"snapshots"`
}

type periodDTO struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (h *Handler) Snapshots(w http.ResponseWriter, r *http.Request) {
	snaps, from, to, ok := h.loadRange(w, r, 24*time.Hour)
	if !ok {
		return
	}
	items := make([]snapshotItem, 0, len(snaps))
	for _, s := range snaps {
		item := snapshotItem{
			TakenAt:            formatJST(s.TakenAt),
			BlockStartedAt:     formatJST(s.BlockStartedAt),
			SessionTokens:      s.TokensUsed,
			Tokens:             toTokenBreakdown(s.Tokens),
			SessionRatio:       s.UsageRatio,
			WeeklyTokens:       s.WeeklyTokens,
			WeeklySonnetTokens: s.WeeklySonnetTokens,
		}
		if len(s.WeeklyModelBreakdown) > 0 {
			item.WeeklyModelBreakdown = make(map[string]tokenBreakdown, len(s.WeeklyModelBreakdown))
			for model, bd := range s.WeeklyModelBreakdown {
				item.WeeklyModelBreakdown[model] = toTokenBreakdown(bd)
			}
		}
		if s.BlockEndedAt != nil {
			item.BlockEndedAt = formatJST(*s.BlockEndedAt)
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, snapshotsResponse{
		Period:    newPeriod(from, to),
		Snapshots: items,
	})
}

// --- /usage/blocks ---

type blockItem struct {
	Start  string  `json:"start"`
	End    string  `json:"end,omitempty"`
	Tokens int     `json:"tokens"`
	Ratio  float64 `json:"ratio"`
}

type weeklyDTO struct {
	TotalTokens int     `json:"total_tokens"`
	Limit       int     `json:"limit,omitempty"`
	Ratio       float64 `json:"ratio,omitempty"`
}

type blocksResponse struct {
	Period periodDTO   `json:"period"`
	Blocks []blockItem `json:"blocks"`
	Weekly weeklyDTO   `json:"weekly"`
}

func (h *Handler) Blocks(w http.ResponseWriter, r *http.Request) {
	snaps, from, to, ok := h.loadRange(w, r, 7*24*time.Hour)
	if !ok {
		return
	}
	aggs := AggregateBlocks(snaps)
	items := make([]blockItem, 0, len(aggs))
	for _, b := range aggs {
		item := blockItem{
			Start:  formatJST(b.Start),
			Tokens: b.Tokens,
			Ratio:  b.Ratio,
		}
		if b.End != nil {
			item.End = formatJST(*b.End)
		}
		items = append(items, item)
	}
	weeklyTotal := SumWeeklyTokens(snaps)
	weekly := weeklyDTO{TotalTokens: weeklyTotal, Limit: h.cfg.WeeklyLimit}
	if h.cfg.WeeklyLimit > 0 {
		weekly.Ratio = float64(weeklyTotal) / float64(h.cfg.WeeklyLimit)
	}
	writeJSON(w, http.StatusOK, blocksResponse{
		Period: newPeriod(from, to),
		Blocks: items,
		Weekly: weekly,
	})
}

// --- /usage/daily ---

type dailyItem struct {
	Date   string `json:"date"`
	Tokens int    `json:"tokens"`
	Blocks int    `json:"blocks"`
}

type dailyResponse struct {
	Period periodDTO   `json:"period"`
	Daily  []dailyItem `json:"daily"`
}

func (h *Handler) Daily(w http.ResponseWriter, r *http.Request) {
	snaps, from, to, ok := h.loadRange(w, r, 7*24*time.Hour)
	if !ok {
		return
	}
	daily := AggregateDaily(AggregateBlocks(snaps))
	items := make([]dailyItem, 0, len(daily))
	for _, d := range daily {
		items = append(items, dailyItem{Date: d.Date, Tokens: d.Tokens, Blocks: d.Blocks})
	}
	writeJSON(w, http.StatusOK, dailyResponse{
		Period: newPeriod(from, to),
		Daily:  items,
	})
}

// --- /usage/summary ---

type summaryResponse struct {
	Window         string    `json:"window"`
	Current        periodSum `json:"current"`
	Previous       periodSum `json:"previous"`
	DeltaRatio     float64   `json:"delta_ratio"` // (current-previous)/previous, 0 if previous=0
	WeeklySonnet   int       `json:"weekly_sonnet_tokens,omitempty"`
}

type periodSum struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Tokens int    `json:"tokens"`
}

func windowDuration(w string) (time.Duration, error) {
	switch w {
	case "", "week":
		return 7 * 24 * time.Hour, nil
	case "month":
		return 30 * 24 * time.Hour, nil
	case "6month":
		return 180 * 24 * time.Hour, nil
	case "year":
		return 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("window must be week|month|6month|year, got %q", w)
	}
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	win := r.URL.Query().Get("window")
	if win == "" {
		win = "week"
	}
	d, err := windowDuration(win)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	now := time.Now().UTC()
	curFrom := now.Add(-d)
	prevFrom := curFrom.Add(-d)

	curSnaps, err := h.repo.ListBetween(r.Context(), curFrom, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	prevSnaps, err := h.repo.ListBetween(r.Context(), prevFrom, curFrom)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	curTokens := sumBlockTokens(AggregateBlocks(curSnaps))
	prevTokens := sumBlockTokens(AggregateBlocks(prevSnaps))

	var delta float64
	if prevTokens > 0 {
		delta = float64(curTokens-prevTokens) / float64(prevTokens)
	}

	writeJSON(w, http.StatusOK, summaryResponse{
		Window:       win,
		Current:      periodSum{From: formatJST(curFrom), To: formatJST(now), Tokens: curTokens},
		Previous:     periodSum{From: formatJST(prevFrom), To: formatJST(curFrom), Tokens: prevTokens},
		DeltaRatio:   delta,
		WeeklySonnet: latestWeeklySonnet(curSnaps),
	})
}

func sumBlockTokens(aggs []BlockAgg) int {
	var sum int
	for _, b := range aggs {
		sum += b.Tokens
	}
	return sum
}

func latestWeeklySonnet(snaps []repository.Snapshot) int {
	var max int
	for _, s := range snaps {
		if s.WeeklySonnetTokens > max {
			max = s.WeeklySonnetTokens
		}
	}
	return max
}
