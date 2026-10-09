package activity

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Harnesses can separate the prompt glyph and input with non-breaking spaces.
func hasPrompt(line string, marker rune) bool {
	r, size := utf8.DecodeRuneInString(line)
	if r != marker {
		return false
	}
	if len(line) == size {
		return true
	}
	next, _ := utf8.DecodeRuneInString(line[size:])
	return unicode.IsSpace(next)
}

// Codex renders empty placeholders dim and user input in the normal style.
// Keep only this style bit during parsing; never store the text or styling.
// Without styling, absent shortcuts alone cannot distinguish a placeholder.
func composerHasDraft(lines []string) bool {
	return inputHasDraft(lines, '›', false)
}

// Claude's bordered composer can contain unstyled text; Codex needs styling
// when its empty-composer shortcut hint is absent.
func inputHasDraft(lines []string, prompt rune, allowPlain bool) bool {
	styled := strings.Contains(strings.Join(lines, ""), "\x1b[")
	if !styled && !allowPlain {
		return false
	}
	dim := false
	for i, line := range lines {
		afterPrompt := i > 0
		for len(line) > 0 {
			var loc []int
			if strings.HasPrefix(line, "\x1b") {
				loc = ansi.FindStringIndex(line)
			}
			if loc != nil && loc[0] == 0 {
				sequence := line[:loc[1]]
				if strings.HasPrefix(sequence, "\x1b[") && strings.HasSuffix(sequence, "m") {
					codes := strings.Split(sequence[2:len(sequence)-1], ";")
					for j := 0; j < len(codes); j++ {
						code, _ := strconv.Atoi(codes[j])
						switch code {
						case 0, 22:
							dim = false
						case 2:
							dim = true
						case 38, 48, 58:
							if j+1 < len(codes) {
								if codes[j+1] == "2" {
									j += 4
								} else if codes[j+1] == "5" {
									j += 2
								}
							}
						}
					}
				}
				line = line[loc[1]:]
				continue
			}
			r, size := utf8.DecodeRuneInString(line)
			line = line[size:]
			if !afterPrompt {
				if r == prompt {
					afterPrompt = true
				}
				continue
			}
			if !unicode.IsSpace(r) && (!styled || !dim) {
				return true
			}
		}
	}
	return false
}
