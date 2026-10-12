package activity

import (
	"strings"
	"testing"
)

func piScreen(body string) string {
	return "Example output\n" + claudeRule + "\n" + body + "\n" + claudeRule + "\n~/example (main)\n↑1.2k ↓20 $0.002 3.5%/200k (auto)    example-model\n"
}

func TestPiEditorAndDialogs(t *testing.T) {
	for _, tt := range []struct{ name, screen, interaction string }{
		{"empty", piScreen("\x1b[7m \x1b[0m"), ""},
		{"draft", piScreen("Example\x1b[7m \x1b[0m"), "draft"},
		{"multiline", piScreen("\nExample\nSecond line"), "draft"},
		{"unicode", piScreen("\u00a0例"), "draft"},
		{"extension status", piScreen("Example") + "extension ready\n", "draft"},
		{"working border", strings.Replace(piScreen("Example"), claudeRule, "── Working… ────────────", 1), "draft"},
		{"scroll border", strings.Replace(piScreen("Example"), claudeRule, "──────── ↑ 2 more ──────", 1), "draft"},
		{"no footer", claudeRule + "\nExample\n" + claudeRule, ""},
		{"unknown footer", strings.Replace(piScreen("Example"), "3.5%/200k", "context unknown", 1), ""},
		{"transcript", piScreen(" ") + "extra transcript\n" + strings.Repeat("line\n", 5), ""},
		{"input", piScreen("\nChoose a name\n\n> \n\nenter submit  esc cancel\n"), "question"},
		{"selector", piScreen("\nChoose an option\n\n→ First\n  Second\n\n↑↓ navigate  enter select  esc cancel\n"), "input"},
		{"typed controls", piScreen("enter submit  esc cancel"), "draft"},
		{"old input", "Choose a name\nenter submit  esc cancel\n" + piScreen(" "), ""},
		{"no process inference", piScreen("sleep 10 &"), "draft"},
		{"unknown context after compaction", strings.Replace(piScreen("Example"), "3.5%/200k", "?/200k", 1), "draft"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			o := Pi(tt.screen)
			if o.Interaction != tt.interaction || o.State != "unsupported" || o.Count != 0 || o.Agents != 0 {
				t.Fatal(o)
			}
		})
	}
}

func FuzzPi(f *testing.F) {
	f.Add(piScreen("Example"))
	f.Fuzz(func(t *testing.T, text string) {
		o := Pi(text)
		if o.State != "unsupported" || o.Count != 0 || o.Agents != 0 {
			t.Fatal(o)
		}
	})
}
