// Package activity interprets live UI signals without assigning agent lifecycle state.
package activity

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Observation struct {
	State       string `json:"state"`
	Count       int    `json:"count,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Interaction string `json:"interaction,omitempty"`
}

func Unknown(reason string) Observation { return Observation{State: "unknown", Reason: reason} }

var (
	ansi           = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(?:\x07|\x1b\\))`)
	contextStatus  = regexp.MustCompile(`(?i)(\bContext\s+\d+%\s+used\b|\b\d+% context(?: window)? left\b)`)
	summary        = regexp.MustCompile(`^(\d+) background terminal(s?) running(.*)$`)
	workingSummary = regexp.MustCompile(`^[•◦] .+esc to interrupt.*?[·•] (\d+ background terminals? running.*)$`)
	clippedSummary = regexp.MustCompile(`^\d+(?:\s|$)`)
	questions      = regexp.MustCompile(`^\? [1-9]\d* questions?(?: · .+)?$`)
)

// Codex separates the interaction from whether the shell count is known.
// Recognition is anchored to the current composer or final question controls.
func Codex(text string) Observation {
	raw := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	lines := make([]string, len(raw))
	status, last := -1, -1
	for i, line := range raw {
		lines[i] = strings.TrimSpace(ansi.ReplaceAllString(line, ""))
		if lines[i] != "" {
			last = i
		}
		if contextStatus.MatchString(lines[i]) {
			status = i
		}
	}
	// Expanded asynchronous questions replace the composer and its status line.
	if last >= 0 && last > status {
		hint := strings.ToLower(lines[last])
		if strings.Contains(hint, "submit") && strings.Contains(hint, "skip") && strings.Contains(hint, "main prompt") {
			o := Unknown("question editor hides the shell footer")
			o.Interaction = "question"
			for i := last - 1; i >= 0; i-- {
				if lines[i] != "• Queued follow-up inputs" {
					continue
				}
				end := i - 1
				for end >= 0 && lines[end] == "" {
					end--
				}
				if end >= 0 {
					if count, ok := backgroundCount(activityBlock(lines, end)); ok {
						o.State, o.Count, o.Reason = "known", count, ""
					}
				}
				break
			}
			return o
		}
	}
	if status < 0 || last-status > 3 {
		return Unknown("live footer unavailable")
	}
	prompt := -1
	for i := status - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "› ") || lines[i] == "›" {
			prompt = i
			break
		}
	}
	if prompt < 0 {
		return Unknown("live composer unavailable")
	}
	// Only inspect controls below the composer, never phrases typed by the user.
	hints := strings.ToLower(strings.Join(lines[status:], " "))
	for _, hint := range []string{"showing detailed transcript", "pgup/pgdn", "↑/↓ to scroll"} {
		if strings.Contains(hints, hint) {
			o := Unknown("history view")
			o.Interaction = "history"
			return o
		}
	}
	for _, hint := range []string{"esc to close", "esc to cancel", "enter to confirm", "enter to select"} {
		if strings.Contains(hints, hint) {
			o := Unknown("overlay view")
			o.Interaction = "menu"
			return o
		}
	}
	o := Unknown("footer boundary unavailable")
	shortcuts := strings.Contains(hints, "for shortcuts")
	queue := strings.Contains(hints, "to queue")
	if queue || (!shortcuts && composerHasDraft(raw[prompt:status])) {
		o.Interaction = "draft"
	}
	if !shortcuts && o.Interaction == "" {
		return o
	}
	end := prompt - 1
	for end >= 0 && lines[end] == "" {
		end--
	}
	// Collapsed questions remain actionable while the model works or the user types.
	if end >= 1 && strings.HasSuffix(lines[end], " to answer") && questions.MatchString(lines[end-1]) {
		o.Interaction = "question"
		o.Reason = "pending question obscures the footer boundary"
		end -= 2
		for end >= 0 && lines[end] == "" {
			end--
		}
		if end >= 0 && lines[end] == "• Queued follow-up inputs" {
			end--
			for end >= 0 && lines[end] == "" {
				end--
			}
		}
	}
	if end < 0 {
		if prompt >= 3 && o.Interaction != "question" {
			o.State, o.Reason = "known", ""
		}
		return o
	}
	block := activityBlock(lines, end)
	if count, ok := backgroundCount(block); ok {
		o.State, o.Count, o.Reason = "known", count, ""
		return o
	}
	if strings.Contains(strings.ToLower(block), "background terminal") || clippedSummary.MatchString(block) || strings.Contains(block, "esc to interrupt") {
		o.Reason = "unrecognized or clipped activity row"
		return o
	}
	if prompt-end-1 >= 2 && o.Interaction != "question" {
		o.State, o.Reason = "known", ""
	}
	return o
}

func activityBlock(lines []string, end int) string {
	start := end
	for start > 0 && end-start < 5 && lines[start-1] != "" {
		start--
	}
	// A working tip may follow the status row without a blank separator.
	if m := workingSummary.FindStringSubmatch(lines[start]); m != nil {
		return m[1]
	}
	return strings.Join(lines[start:end+1], " ")
}

func backgroundCount(block string) (int, bool) {
	m := summary.FindStringSubmatch(block)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	// A complete count phrase suffices when only trailing shortcut help is clipped.
	ok := err == nil && n >= 1 && n <= 10000 && (n == 1) == (m[2] == "") && strings.HasPrefix(" · /ps to view · /stop to close", m[3])
	return n, ok
}

func (o Observation) Label() string {
	parts := []string{}
	switch o.Interaction {
	case "question":
		parts = append(parts, "Reply needed")
	case "draft":
		parts = append(parts, "Draft prompt")
	case "input":
		parts = append(parts, "Input needed")
	case "menu":
		parts = append(parts, "Menu open")
	case "history":
		parts = append(parts, "Viewing history")
	}
	if o.State == "known" && o.Count > 0 {
		label := fmt.Sprintf("%d background shells", o.Count)
		if o.Count == 1 {
			label = "1 background shell"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, " · ")
}
