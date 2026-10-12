package activity

import "strings"

// BlockedInput refines Herdr's blocked state only when the live dialog has
// recognized controls and choices. It never submits or answers the dialog.
func BlockedInput(text, harness string) string {
	lines := plainLines(text)
	if len(lines) == 0 {
		return "input"
	}
	hints := strings.ToLower(strings.Join(lines[max(0, len(lines)-3):], " "))
	if !strings.Contains(hints, "esc to cancel") ||
		(!strings.Contains(hints, "enter to select") && !strings.Contains(hints, "enter to confirm")) {
		return "input"
	}
	start := max(0, len(lines)-40)
	for i := len(lines) - 2; i >= start; i-- {
		if horizontalRule(lines[i]) {
			start = i + 1
			break
		}
	}
	body := strings.ToLower(strings.Join(lines[start:], "\n"))
	if harness == "codex" && strings.Contains(body, "yes, proceed") && strings.Contains(body, "no, and tell codex") {
		return "approval"
	}
	if harness == "claude" {
		for _, header := range []string{"do you want to proceed?", "do you want to make this edit", "do you want to allow", "yes, i trust this folder"} {
			if strings.Contains(body, header) && strings.Contains(body, "yes") {
				return "approval"
			}
		}
		// Multi-question navigation is specific to the question form. A generic
		// arrow-select menu or Yes/No dialog alone does not establish a question.
		if strings.Contains(hints, "tab/arrow keys to navigate") ||
			(strings.Contains(body, "type something") && strings.Contains(hints, "to navigate")) {
			return "question"
		}
	}
	return "input"
}

func plainLines(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	for i := range raw {
		raw[i] = strings.TrimSpace(ansi.ReplaceAllString(raw[i], ""))
	}
	for len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	return raw
}
