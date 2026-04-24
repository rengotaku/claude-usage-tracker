// Package setup parses Claude web /usage page text and recommends env vars.
package setup

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Result holds values parsed from the Claude web /usage page text.
type Result struct {
	Plan            string       // display name, e.g. "Max (5x)"
	Tier            string       // tier ID, e.g. "default_claude_max_5x"
	WeeklyResetDay  time.Weekday
	WeeklyResetHour int          // hour as shown in web UI
	HasReset        bool
}

// resetRe matches "Resets Thu 7:00 AM" but not "Resets in 4 hr 9 min".
var resetRe = regexp.MustCompile(`(?i)Resets\s+(Sun|Mon|Tue|Wed|Thu|Fri|Sat)\s+(\d+):(\d+)\s+(AM|PM)`)

var dayMap = map[string]time.Weekday{
	"sun": time.Sunday,
	"mon": time.Monday,
	"tue": time.Tuesday,
	"wed": time.Wednesday,
	"thu": time.Thursday,
	"fri": time.Friday,
	"sat": time.Saturday,
}

// ParseWebText extracts plan and weekly reset info from the Claude web /usage page text.
// The hour in Result is as displayed in the web UI (user's local timezone).
func ParseWebText(text string) Result {
	var r Result
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch {
		case strings.Contains(line, "Max (20x)"):
			r.Plan = "Max (20x)"
			r.Tier = "default_claude_max_20x"
		case strings.Contains(line, "Max (5x)"):
			r.Plan = "Max (5x)"
			r.Tier = "default_claude_max_5x"
		case line == "Pro":
			r.Plan = "Pro"
			r.Tier = "default_claude_pro"
		}

		if !r.HasReset {
			if m := resetRe.FindStringSubmatch(line); m != nil {
				hour, _ := strconv.Atoi(m[2])
				ampm := strings.ToUpper(m[4])
				if ampm == "PM" && hour != 12 {
					hour += 12
				} else if ampm == "AM" && hour == 12 {
					hour = 0
				}
				r.WeeklyResetDay = dayMap[strings.ToLower(m[1])]
				r.WeeklyResetHour = hour
				r.HasReset = true
			}
		}
	}
	return r
}
