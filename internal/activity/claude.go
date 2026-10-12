package activity

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	claudeShells     = regexp.MustCompile(`^([1-9][0-9]*) shell(s?)$`)
	claudeMode       = regexp.MustCompile(`^(?:⏵⏵ (?:auto mode|accept edits|bypass permissions) on|⏸ plan mode on|\? for shortcuts)(?:\s|$)`)
	claudeWaiting    = regexp.MustCompile(`^[^\p{L}\p{N}\s] Waiting for ([1-9][0-9]*) background agent(s?) to finish$`)
	claudeCountStart = regexp.MustCompile(`^[1-9][0-9]* shells?(?: ·|$)`)
)

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
	if top < 0 || bottom-top < 2 {
		return o
	}
	for i := bottom + 1; i <= last; i++ {
		if lines[i] == "⏺ main" {
			last = i - 1
			break
		}
	}
	for last > bottom && lines[last] == "" {
		last--
	}
	if last-bottom > 8 {
		return o
	}
	footer := strings.ToLower(strings.Join(lines[bottom+1:last+1], " "))
	native := claudeNativeFooter(lines[bottom+1 : last+1])
	if native == "" {
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
	// Join only native control rows, including their wrapped continuations.
	// Missing counts stay unknown:
	// narrow layouts may hide them, so absence cannot establish zero.
	if count, ok := claudeShellCount(native); ok {
		o.State, o.Count, o.Reason = "known", count, ""
	}
	end := top - 1
	for end >= 0 && lines[end] == "" {
		end--
	}
	for start := end; start >= max(0, end-2); start-- {
		if lines[start] == "" {
			break
		}
		if m := claudeWaiting.FindStringSubmatch(strings.Join(lines[start:end+1], " ")); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n <= 10000 && (n == 1) == (m[2] == "") {
				o.Agents = n
			}
			break
		}
	}
	return o
}

func claudeNativeFooter(lines []string) string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	// A recognized permission-mode prefix anchors the control row even if its
	// trailing shortcut is clipped. Never merge preceding custom status lines.
	for i := len(lines) - 1; i >= max(0, len(lines)-4); i-- {
		joined := strings.Join(strings.Fields(strings.Join(lines[i:], " ")), " ")
		if claudeMode.MatchString(joined) {
			return joined
		}
		if claudeCountStart.MatchString(joined) && strings.HasSuffix(joined, "← for agents") {
			// Without a mode prefix, only join rows that visibly continue the
			// control row. A standalone custom status above it stays separate.
			continued := true
			for _, line := range lines[i+1:] {
				if !strings.HasPrefix(line, "· ") {
					continued = false
				}
			}
			if continued {
				return joined
			}
		}
	}
	if len(lines) > 0 && claudeFooter(lines[len(lines)-1]) {
		return lines[len(lines)-1]
	}
	return ""
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
