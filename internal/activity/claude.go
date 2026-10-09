package activity

import (
	"regexp"
	"strconv"
	"strings"
)

var claudeShells = regexp.MustCompile(`^([1-9][0-9]*) shell(s?)$`)

// Claude detects drafts and explicit shell counts in the current native footer.
// Require the current bordered prompt box and a recognizable footer so transcript
// prompts, permission choices, and other uses of the same chevron are excluded.
func Claude(text string) Observation {
	o := Unknown("Claude shell count not visible")
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
	if !claudeFooter(footer) {
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
	if prompt >= bottom || !hasPrompt(lines[prompt], '❯') {
		return o
	}
	if inputHasDraft(raw[prompt:bottom], '❯', true) {
		o.Interaction = "draft"
	}
	// Custom status lines and old turn summaries are not the live task inventory.
	// Only inspect the final native control row. Missing counts stay unknown:
	// narrow layouts may hide them, so absence cannot establish zero.
	if claudeFooter(lines[last]) {
		if count, ok := claudeShellCount(lines[last]); ok {
			o.State, o.Count, o.Reason = "known", count, ""
		}
	}
	return o
}

func claudeFooter(line string) bool {
	line = strings.TrimSpace(line)
	return strings.Contains(line, "shift+tab to cycle") ||
		strings.Contains(line, "? for shortcuts") || strings.HasSuffix(line, "← for agents")
}

func claudeShellCount(line string) (int, bool) {
	count, found := 0, false
	for _, part := range strings.Split(line, " · ") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, "shell") {
			continue
		}
		m := claudeShells.FindStringSubmatch(part)
		if m == nil || found {
			return 0, false
		}
		n, err := strconv.Atoi(m[1])
		if err != nil || n > 10000 || (n == 1) != (m[2] == "") {
			return 0, false
		}
		count, found = n, true
	}
	return count, found
}

func horizontalRule(line string) bool {
	return len([]rune(line)) >= 10 && strings.Trim(line, "─") == ""
}
