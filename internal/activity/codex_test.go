package activity

import (
	"strings"
	"testing"
)

const footer = "\n\n\n› Ask Codex to do anything\n\n  GPT-6 · Context 42% used\n  ? for shortcuts\n"
const one = "1 background terminal running · /ps to view · /stop to close"

func TestCodexBoundaries(t *testing.T) {
	tests := []struct {
		name, text, state string
		count             int
	}{
		{"zero", "• Finished the example." + footer, "known", 0},
		{"empty", "" + footer, "known", 0},
		{"one", "Example output\n\n  " + one + footer, "known", 1},
		{"many", "\n\n12 background terminals running · /ps to view · /stop to close" + footer, "known", 12},
		{"wrapped", "\n\n2 background terminals running · /ps to view ·\n /stop to close" + footer, "known", 2},
		{"working", "\n\n• Working (1m · esc to interrupt) · " + one + footer, "known", 1},
		{"working tip", "\n\n• Working (1m • esc to interrupt) · " + one + "\n  Tip: Try a shorter prompt." + footer, "known", 1},
		{"working hidden", "\n\n• Working (1m · esc to interrupt)" + footer, "unknown", 0},
		{"ansi", "\n\n\x1b[2m" + one + "\x1b[0m" + footer, "known", 1},
		{"quoted mention", "• Example: " + one + footer, "unknown", 0},
		{"old mention", "• Example: " + one + "\n\n• Finished." + footer, "known", 0},
		{"summary only", one, "unknown", 0},
		{"fake transcript prompt", "› Example\n" + one + "\n• Finished." + footer, "unknown", 0},
		{"typed summary", "• Finished.\n\n\n› " + one + "\n\nContext 42% used\n? for shortcuts", "known", 0},
		{"menu", "\n\n" + one + footer + "esc to close", "unknown", 0},
		{"history", "\n\n" + one + footer + "↑/↓ to scroll", "unknown", 0},
		{"clipped help", "\n\n1 background terminal running · /ps to" + footer, "known", 1},
		{"clipped beginning", "\n\n1 backg" + footer, "unknown", 0},
		{"wrong plural", "\n\n1 background terminals running · /ps to view · /stop to close" + footer, "unknown", 0},
		{"invalid zero summary", "\n\n0 background terminals running · /ps to view · /stop to close" + footer, "unknown", 0},
		{"overflow", "\n\n99999999999999999999999 background terminals running · /ps to view · /stop to close" + footer, "unknown", 0},
		{"unknown footer", "\n\n" + one + strings.ReplaceAll(footer, "Context 42% used", "custom status"), "unknown", 0},
		{"no shortcuts", "\n\n" + one + strings.ReplaceAll(footer, "? for shortcuts", ""), "unknown", 0},
		{"missing boundary", "text\ntext\ntext\n› Prompt\nContext 42% used\n? for shortcuts", "unknown", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Codex(tt.text)
			if got.State != tt.state || got.Count != tt.count {
				t.Fatalf("got %+v, want %s/%d", got, tt.state, tt.count)
			}
		})
	}
}

func TestCodexInteractions(t *testing.T) {
	status := "\n\n  Context 42% used\n"
	draft := "\n\n\n\x1b[0m› \x1b[48;2;57;57;71mWrite a short example\x1b[0m" + status
	question := "• Queued follow-up inputs\n\nWhich option?\n\nType your answer\n\nenter submit   ctrl+] skip   shift+→ main prompt\n"
	for _, tt := range []struct {
		name, text, interaction, state string
		count                          int
	}{
		{"draft", "• Complete." + draft, "draft", "known", 0},
		{"draft and shell", "\n\n" + one + draft, "draft", "known", 1},
		{"multiline draft", "• Complete.\n\n\n\x1b[0m› First line\n  Second line\x1b[0m" + status, "draft", "known", 0},
		{"draft with modal words", "• Complete.\n\n\n\x1b[0m› explain esc to close and enter to confirm\x1b[0m" + status, "draft", "known", 0},
		{"queue hint", "• Complete.\n\n\n› a queued draft" + status + "tab to queue message", "draft", "known", 0},
		{"dim placeholder", "• Complete.\n\n\n\x1b[0m› \x1b[2mAsk anything\x1b[0m" + status, "", "unknown", 0},
		{"dim RGB placeholder", "• Complete.\n\n\n\x1b[0m› \x1b[2;38;2;200;22;0mAsk anything\x1b[0m" + status, "", "unknown", 0},
		{"normal indexed color", "• Complete.\n\n\n\x1b[0m› \x1b[38;5;2mExample\x1b[0m" + status, "draft", "known", 0},
		{"shortcuts placeholder", "• Complete." + footer, "", "known", 0},
		{"default context status", "• Complete." + strings.ReplaceAll(footer, "Context 42% used", "58% context left"), "", "known", 0},
		{"expanded question", question, "question", "unknown", 0},
		{"expanded with shell", "\n\n" + one + "\n\n" + question, "question", "known", 1},
		{"expanded while working", "• Working (1s • esc to interrupt)\n\n" + question, "question", "unknown", 0},
		{"expanded typed reply", strings.ReplaceAll(question, "Type your answer", "An answer draft"), "question", "unknown", 0},
		{"collapsed question", "• Queued follow-up inputs\n  ? 2 questions · 15s\n  shift+← to answer" + footer, "question", "unknown", 0},
		{"collapsed with draft", "• Queued follow-up inputs\n  ? 1 question\n  shift+← to answer" + draft, "question", "unknown", 0},
		{"collapsed and shell", "\n\n" + one + "\n\n• Queued follow-up inputs\n  ? 1 question\n  shift+← to answer" + footer, "question", "known", 1},
		{"question marker typed in draft", "• Complete.\n\n\n\x1b[0m› ? 2 questions\n  shift+← to answer\x1b[0m" + status, "draft", "known", 0},
		{"question controls typed in draft", "• Complete.\n\n\n\x1b[0m› enter submit   ctrl+] skip   shift+→ main prompt\x1b[0m" + status, "draft", "known", 0},
		{"menu", "\n\n" + one + footer + "esc to close", "menu", "unknown", 0},
		{"history", "\n\n" + one + footer + "↑/↓ to scroll", "history", "unknown", 0},
		{"clipped phrase", "\n\n1 background terminal run" + footer, "", "unknown", 0},
		{"summary unexpected suffix", "\n\n" + one + " quoted" + footer, "", "unknown", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			o := Codex(tt.text)
			if o.Interaction != tt.interaction || o.State != tt.state || o.Count != tt.count {
				t.Fatalf("got %+v; want %s %s %d", o, tt.interaction, tt.state, tt.count)
			}
		})
	}
}

func TestQuietUnknownAndUsefulLabels(t *testing.T) {
	if Unknown("unavailable").Label() != "" {
		t.Fatal("unknown should not create a badge")
	}
	if got := (Observation{State: "unknown", Interaction: "question"}).Label(); got != "Reply needed" {
		t.Fatal(got)
	}
	if got := (Observation{State: "known", Count: 1, Interaction: "draft"}).Label(); got != "Draft prompt · 1 background shell" {
		t.Fatal(got)
	}
}

func FuzzCodex(f *testing.F) {
	f.Add("\n\n" + one + footer)
	f.Add("\x1b[2m› anything\n")
	f.Fuzz(func(t *testing.T, text string) {
		o := Codex(text)
		if o.Count < 0 || o.Count > 10000 || (o.State != "known" && o.Count != 0) {
			t.Fatalf("invalid result: %+v", o)
		}
	})
}
