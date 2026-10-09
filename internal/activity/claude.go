package activity

import "strings"

// Claude detects draft text only. Its background-shell inventory is unsupported.
// Require the current bordered prompt box and a recognizable footer so transcript
// prompts, permission choices, and other uses of the same chevron are excluded.
func Claude(text string) Observation {
	o := Observation{State: "unsupported", Reason: "Claude shell counts unsupported"}
	raw := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	lines := make([]string, len(raw))
	top, bottom, last := -1, -1, -1
	for i, line := range raw {
		lines[i] = strings.TrimSpace(ansi.ReplaceAllString(line, ""))
		if lines[i] != "" {
			last = i
		}
		if horizontalRule(lines[i]) {
			top, bottom = bottom, i
		}
	}
	if top < 0 || bottom-top < 2 || last-bottom > 8 {
		return o
	}
	footer := strings.ToLower(strings.Join(lines[bottom+1:], " "))
	if !strings.Contains(footer, "shift+tab to cycle") && !strings.Contains(footer, "? for shortcuts") {
		return o
	}
	for _, hint := range []string{"esc to cancel", "esc to close", "enter to select", "enter to confirm", "showing detailed transcript", "↑↓ scroll"} {
		if strings.Contains(footer, hint) {
			return o
		}
	}
	prompt := top + 1
	for prompt < bottom && lines[prompt] == "" {
		prompt++
	}
	if prompt >= bottom || (lines[prompt] != "❯" && !strings.HasPrefix(lines[prompt], "❯ ")) {
		return o
	}
	if inputHasDraft(raw[prompt:bottom], '❯', true) {
		o.Interaction = "draft"
	}
	return o
}

func horizontalRule(line string) bool {
	return len([]rune(line)) >= 10 && strings.Trim(line, "─") == ""
}
