package activity

import (
	"regexp"
	"strings"
)

var piContext = regexp.MustCompile(`(?:^|\s)(?:[0-9]+(?:\.[0-9]+)?%|\?)/[0-9]+(?:\.[0-9]+)?[kM]?(?:\s|$)`)

// Pi recognizes the stock v0.87.1 bordered editor and built-in extension dialogs.
// Its bash tool has no background task registry: never infer a background count
// from an old tool result, an ampersand in a command, or a generic extension row.
func Pi(text string) Observation {
	o := Observation{State: "unsupported", Reason: "stock Pi has no background-task inventory"}
	lines := plainLines(text)
	stats := -1
	for i := len(lines) - 1; i >= max(0, len(lines)-4); i-- {
		if piContext.MatchString(lines[i]) {
			stats = i
			break
		}
	}
	if stats < 1 {
		return o
	}
	// The stock footer has cwd then token/context stats. Arbitrary extension
	// footers are deliberately not recognized as a composer boundary.
	cwd := lines[stats-1]
	if !strings.HasPrefix(cwd, "/") && !strings.HasPrefix(cwd, "~") {
		return o
	}
	bottom := stats - 2
	for bottom >= 0 && lines[bottom] == "" {
		bottom--
	}
	if bottom < 0 || !piBorder(lines[bottom]) {
		return o
	}
	top := bottom - 1
	for top >= 0 && !piBorder(lines[top]) {
		top--
	}
	if top < 0 || bottom-top < 2 {
		return o
	}
	body := lines[top+1 : bottom]
	end := len(body) - 1
	for end >= 0 && body[end] == "" {
		end--
	}
	if end < 0 {
		return o
	}
	hints := strings.ToLower(body[end])
	// Pi's extension input has dedicated submit/cancel controls. Selectors
	// use the same chrome for arbitrary menus, so leave their purpose generic.
	dialog := body[0] == "" && len(body) >= 5
	if dialog && (hints == "enter submit  esc cancel" || hints == "enter submit esc cancel") {
		o.Interaction = "question"
		return o
	}
	if dialog && strings.Contains(hints, "↑↓ navigate") && strings.Contains(hints, "enter select") && strings.Contains(hints, "esc cancel") {
		o.Interaction = "input"
		return o
	}
	for _, line := range body {
		if line != "" {
			o.Interaction = "draft"
			break
		}
	}
	return o
}

func piBorder(line string) bool {
	return horizontalRule(line) || (len([]rune(line)) >= 10 && strings.HasPrefix(line, "──") && strings.HasSuffix(line, "──"))
}
