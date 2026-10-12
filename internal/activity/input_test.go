package activity

import "testing"

func TestBlockedInput(t *testing.T) {
	for _, tt := range []struct{ name, harness, screen, want string }{
		{"Claude approval", "claude", claudeRule + "\nBash command\nDo you want to proceed?\n❯ 1. Yes\n2. No\nEnter to select · Esc to cancel", "approval"},
		{"Claude question", "claude", claudeRule + "\nChoose an example\n❯ 1. First\n2. Second\nEnter to select · Tab/Arrow keys to navigate · Esc to cancel", "question"},
		{"Claude ambiguous", "claude", claudeRule + "\nSelect a model\n❯ 1. First\nEnter to select · ↑/↓ to navigate · Esc to cancel", "input"},
		{"Claude controls missing", "claude", claudeRule + "\nDo you want to proceed?\nYes", "input"},
		{"Claude stale approval", "claude", "Do you want to proceed?\nYes\n" + claudeRule + "\nChoose an example\nEnter to select · Tab/Arrow keys to navigate · Esc to cancel", "question"},
		{"Codex approval", "codex", "Would you like to run the following command?\n› 1. Yes, proceed (y)\n2. No, and tell Codex what to do differently (esc)\nPress enter to confirm or esc to cancel", "approval"},
		{"Codex menu", "codex", "Choose a model\nPress enter to confirm or esc to cancel", "input"},
		{"empty", "claude", "", "input"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := BlockedInput(tt.screen, tt.harness); got != tt.want {
				t.Fatalf("got %s want %s", got, tt.want)
			}
		})
	}
}
