package activity

import (
	"strings"
	"testing"
)

const claudeRule = "────────────────────────────────────────────────────"

func claudeScreen(body string) string {
	return "Example output\n\n" + claudeRule + "\n" + body + "\n" + claudeRule + "\nExample status\n⏵⏵ auto mode on (shift+tab to cycle)\n"
}

func TestClaudeDraft(t *testing.T) {
	for _, tt := range []struct {
		name, screen string
		draft        bool
	}{
		{"empty", claudeScreen("❯ "), false},
		{"typed", claudeScreen("❯ Write an example"), true},
		{"live non-breaking space", claudeScreen("❯\u00a0aaa"), true},
		{"empty non-breaking space", claudeScreen("❯\u00a0"), false},
		{"narrow non-breaking space", claudeScreen("❯\u202fExample"), true},
		{"multiline", claudeScreen("❯ First line\n  Second line"), true},
		{"newline first", claudeScreen("❯ \n  Second line"), true},
		{"whitespace", claudeScreen("❯  \n  "), false},
		{"styled", claudeScreen("\x1b[0m❯ \x1b[0mWrite an example\x1b[0m"), true},
		{"placeholder", claudeScreen("❯ \x1b[2mTry an example\x1b[0m"), false},
		{"input words", claudeScreen("❯ explain esc to cancel"), true},
		{"transcript prompt", "❯ An old prompt\n" + claudeScreen("❯ "), false},
		{"unbordered", "❯ An example\n? for shortcuts", false},
		{"missing upper border", "❯ An example\n" + claudeRule + "\n? for shortcuts", false},
		{"missing footer", claudeRule + "\n❯ An example\n" + claudeRule, false},
		{"permission choice", claudeScreen("❯ 1. Yes") + "enter to select · esc to cancel", false},
		{"content before prompt", claudeScreen("Do you want to proceed?\n❯ 1. Yes"), false},
		{"history", claudeScreen("❯ An example") + "showing detailed transcript", false},
		{"shortcuts footer", strings.ReplaceAll(claudeScreen("❯ Example"), "⏵⏵ auto mode on (shift+tab to cycle)", "? for shortcuts"), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			o := Claude(tt.screen)
			if (o.Interaction == "draft") != tt.draft || o.State != "unsupported" || o.Count != 0 {
				t.Fatalf("unexpected observation: %+v", o)
			}
		})
	}
}

func FuzzClaude(f *testing.F) {
	f.Add(claudeScreen("❯ Example"))
	f.Fuzz(func(t *testing.T, text string) {
		o := Claude(text)
		if o.State != "unsupported" || o.Count != 0 {
			t.Fatalf("invented shell count: %+v", o)
		}
	})
}
