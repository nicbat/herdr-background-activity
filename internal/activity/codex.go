// Package activity interprets live UI signals without assigning agent lifecycle state.
package activity

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Observation struct {
	State  string `json:"state"`
	Count  int    `json:"count,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func Unknown(reason string) Observation { return Observation{State: "unknown", Reason: reason} }

var (
	ansi           = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07))`)
	contextStatus  = regexp.MustCompile(`(?i)\bContext\s+\d+%\s+used\b`)
	summary        = regexp.MustCompile(`^(\d+) background terminal(s?) running · /ps to view · /stop to close$`)
	workingSummary = regexp.MustCompile(`^[•◦] .+esc to interrupt.*? · (\d+ background terminals? running · /ps to view · /stop to close)$`)
	clippedSummary = regexp.MustCompile(`^\d+(?:\s|$)`)
)

// Codex recognizes the v0.160 live composer, not matching text elsewhere in a transcript.
// Alternate layouts and obscured views deliberately return unknown.
func Codex(text string) Observation {
	text = ansi.ReplaceAllString(text, "")
	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	prompt := -1
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
		if strings.HasPrefix(lines[i], "› ") || lines[i] == "›" {
			prompt = i
		}
	}
	if prompt < 3 {
		return Unknown("live composer unavailable")
	}
	status, shortcuts := false, false
	for _, line := range lines[prompt+1:] {
		status = status || contextStatus.MatchString(line)
		shortcuts = shortcuts || strings.Contains(line, "? for shortcuts")
	}
	if !status || !shortcuts {
		return Unknown("live footer unavailable")
	}
	// Modal hints can coexist with the composer. They invalidate negative evidence.
	tail := strings.ToLower(strings.Join(lines[max(0, prompt-10):], " "))
	for _, hint := range []string{"esc to close", "esc to cancel", "enter to confirm", "enter to select", "showing detailed transcript", "pgup/pgdn", "↑/↓ to scroll"} {
		if strings.Contains(tail, hint) {
			return Unknown("overlay or history view")
		}
	}
	end := prompt - 1
	for end >= 0 && lines[end] == "" {
		end--
	}
	if end < 0 {
		return Observation{State: "known"}
	}
	start := end
	for start > 0 && end-start < 5 && lines[start-1] != "" {
		start--
	}
	block := strings.Join(lines[start:end+1], " ")
	if m := workingSummary.FindStringSubmatch(block); m != nil {
		block = m[1]
	}
	if m := summary.FindStringSubmatch(block); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil || n < 1 || n > 10000 || (n == 1) != (m[2] == "") {
			return Unknown("invalid count")
		}
		return Observation{State: "known", Count: n}
	}
	// A clipped summary must never become a false zero.
	if strings.Contains(strings.ToLower(block), "background terminal") {
		return Unknown("unrecognized or clipped summary")
	}
	if clippedSummary.MatchString(block) || strings.Contains(block, "esc to interrupt") {
		return Unknown("possibly clipped activity row")
	}
	// A normal composer reserves blank rows even when there is no summary.
	if prompt-end-1 < 2 {
		return Unknown("footer boundary unavailable")
	}
	return Observation{State: "known"}
}

func (o Observation) Label() string {
	if o.State == "unknown" {
		return "background ?"
	}
	if o.State != "known" || o.Count == 0 {
		return ""
	}
	if o.Count == 1 {
		return "1 background shell"
	}
	return fmt.Sprintf("%d background shells", o.Count)
}
