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
		{"working hidden", "\n\n• Working (1m · esc to interrupt)" + footer, "unknown", 0},
		{"ansi", "\n\n\x1b[2m" + one + "\x1b[0m" + footer, "known", 1},
		{"quoted mention", "• Example: " + one + footer, "unknown", 0},
		{"old mention", "• Example: " + one + "\n\n• Finished." + footer, "known", 0},
		{"summary only", one, "unknown", 0},
		{"fake transcript prompt", "› Example\n" + one + "\n• Finished." + footer, "unknown", 0},
		{"typed summary", "• Finished.\n\n\n› " + one + "\n\nContext 42% used\n? for shortcuts", "known", 0},
		{"menu", "\n\n" + one + footer + "esc to close", "unknown", 0},
		{"history", "\n\n" + one + footer + "↑/↓ to scroll", "unknown", 0},
		{"clipped end", "\n\n1 background terminal running · /ps to" + footer, "unknown", 0},
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
