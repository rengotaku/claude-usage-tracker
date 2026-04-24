package setup_test

import (
	"testing"
	"time"

	"github.com/rengotaku/claude-usage-tracker/internal/setup"
)

const sampleText = `Plan usage limits
Max (5x)
Current session

Resets in 4 hr 9 min

19% used

Weekly limits
Learn more about usage limits

All models

Resets Thu 7:00 AM

7% used

Sonnet only


Resets Thu 7:00 AM

2% used`

func TestParseWebText_Max5x(t *testing.T) {
	r := setup.ParseWebText(sampleText)
	if r.Tier != "default_claude_max_5x" {
		t.Errorf("Tier: want default_claude_max_5x, got %s", r.Tier)
	}
	if r.Plan != "Max (5x)" {
		t.Errorf("Plan: want Max (5x), got %s", r.Plan)
	}
	if !r.HasReset {
		t.Fatal("HasReset should be true")
	}
	if r.WeeklyResetDay != time.Thursday {
		t.Errorf("WeeklyResetDay: want Thursday, got %s", r.WeeklyResetDay)
	}
	if r.WeeklyResetHour != 7 {
		t.Errorf("WeeklyResetHour: want 7, got %d", r.WeeklyResetHour)
	}
}

func TestParseWebText_Max20x(t *testing.T) {
	r := setup.ParseWebText("Max (20x)\nResets Fri 6:00 AM\n")
	if r.Tier != "default_claude_max_20x" {
		t.Errorf("Tier: want default_claude_max_20x, got %s", r.Tier)
	}
	if r.WeeklyResetDay != time.Friday {
		t.Errorf("WeeklyResetDay: want Friday, got %s", r.WeeklyResetDay)
	}
	if r.WeeklyResetHour != 6 {
		t.Errorf("WeeklyResetHour: want 6, got %d", r.WeeklyResetHour)
	}
}

func TestParseWebText_PM(t *testing.T) {
	r := setup.ParseWebText("Max (5x)\nResets Tue 5:00 PM\n")
	if r.WeeklyResetHour != 17 {
		t.Errorf("WeeklyResetHour: want 17, got %d", r.WeeklyResetHour)
	}
	if r.WeeklyResetDay != time.Tuesday {
		t.Errorf("WeeklyResetDay: want Tuesday, got %s", r.WeeklyResetDay)
	}
}

func TestParseWebText_SessionResetIgnored(t *testing.T) {
	// "Resets in X hr Y min" must not be parsed as weekly reset
	text := "Max (5x)\nResets in 4 hr 9 min\n19% used\nResets Thu 7:00 AM\n"
	r := setup.ParseWebText(text)
	if r.WeeklyResetDay != time.Thursday {
		t.Errorf("WeeklyResetDay: want Thursday, got %s", r.WeeklyResetDay)
	}
}

func TestParseWebText_Empty(t *testing.T) {
	r := setup.ParseWebText("")
	if r.HasReset {
		t.Error("HasReset should be false for empty input")
	}
	if r.Tier != "" {
		t.Errorf("Tier should be empty, got %s", r.Tier)
	}
}
