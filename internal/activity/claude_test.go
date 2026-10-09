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
			if (o.Interaction == "draft") != tt.draft || o.State != "unknown" || o.Count != 0 {
				t.Fatalf("unexpected observation: %+v", o)
			}
		})
	}
}

func FuzzClaude(f *testing.F) {
	f.Add(claudeScreen("❯ Example"))
	f.Fuzz(func(t *testing.T, text string) {
		o := Claude(text)
		if (o.State != "known" && o.State != "unknown") || o.Count < 0 || o.Count > 10000 || (o.State != "known" && o.Count != 0) || (o.State == "known" && o.Count == 0) {
			t.Fatalf("invalid shell count: %+v", o)
		}
	})
}

func TestClaudeShellCounts(t *testing.T) {
	for _, tt := range []struct {
		name, footer string
		count        int
	}{
		{"one", "⏵⏵ auto mode on · 1 shell · ← for agents", 1},
		{"several", "⏵⏵ auto mode on · 3 shells · ← for agents", 3},
		{"default mode", "1 shell · ← for agents", 1},
		{"mixed tasks", "⏵⏵ auto mode on · 2 agents · 3 shells · ← for agents", 3},
		{"only agents", "⏵⏵ auto mode on · 2 agents · ← for agents", 0},
		{"generic tasks", "2 tasks · ← for agents", 0},
		{"no count", "⏵⏵ auto mode on (shift+tab to cycle) · ← for agents", 0},
		{"zero", "0 shells · ← for agents", 0},
		{"bad plural", "1 shells · ← for agents", 0},
		{"bad singular", "2 shell · ← for agents", 0},
		{"overflow", "999999999999999999999999 shells · ← for agents", 0},
		{"unreasonable", "10001 shells · ← for agents", 0},
		{"partial count", "⏵⏵ auto mode on · 2 shel · ← for agents", 0},
		{"partial footer", "⏵⏵ auto mode on · 2 shells · ← for", 0},
		{"old summary", "1 shell still running · ← for agents", 0},
		{"duplicate", "1 shell · 2 shells · ← for agents", 0},
		{"custom status", "1 shell\n⏵⏵ auto mode on (shift+tab to cycle)", 0},
		{"menu", "1 shell · ← for agents\nEnter to select · Esc to cancel", 0},
		{"history", "1 shell · ← for agents\nshowing detailed transcript", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			screen := claudeRule + "\n❯\u00a0\n" + claudeRule + "\n" + tt.footer + "\n"
			o := Claude(screen)
			state := "unknown"
			if tt.count > 0 {
				state = "known"
			}
			if o.State != state || o.Count != tt.count {
				t.Fatalf("unexpected observation: %+v", o)
			}
		})
	}
}

func TestClaudeShellAndDraftBoundaries(t *testing.T) {
	native := "⏵⏵ auto mode on · 2 shells · ← for agents"
	screen := claudeRule + "\n❯\u00a0Example\n" + claudeRule + "\n" + native
	o := Claude(screen)
	if o.Label() != "Draft prompt · 2 background shells" {
		t.Fatal(o)
	}
	// A task count typed into the composer or left in history is never live.
	screen = "1 shell still running\n" + claudeScreen("❯ "+native)
	o = Claude(screen)
	if o.Label() != "Draft prompt" || o.State != "unknown" {
		t.Fatal(o)
	}
	screen = "1 shell still running\n" + claudeScreen("❯ ")
	o = Claude(screen)
	if o.Label() != "" || o.State != "unknown" {
		t.Fatal(o)
	}
}
