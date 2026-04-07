// Package tmuxedit implements a tmux popup editor for composing AI agent prompts.
// agentutil.go provides shared helpers for prompt extraction and tmux key sending
// used by individual agent implementations.
package tmuxedit

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// promptMatch holds a regex match result with its line number in the pane.
type promptMatch struct {
	lineNum int
	text    string // capture group 1
}

// matchPromptLines runs the prompt regex against each pane line, returning
// matches with their line numbers for contiguity analysis.
func matchPromptLines(re *regexp.Regexp, paneContent string) []promptMatch {
	paneLines := strings.Split(paneContent, "\n")
	var matches []promptMatch
	for i, line := range paneLines {
		m := re.FindStringSubmatch(line)
		if len(m) >= 2 {
			matches = append(matches, promptMatch{lineNum: i, text: m[1]})
		}
	}
	return matches
}

// joinAllMatches strips noise from all matches and joins the non-empty results
// with newlines. Used when SectionPattern has already scoped to the prompt area.
func joinAllMatches(matches []promptMatch, strips []string) string {
	var lines []string
	for _, m := range matches {
		line := stripNoise(m.text, strips)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// joinLastContiguousBlock takes the last group of matches on consecutive line
// numbers, strips noise from each, and joins the non-empty results with
// newlines. This ensures that only the bottom-most box (the input prompt)
// is captured when multiple box-drawing sections exist in the pane.
func joinLastContiguousBlock(matches []promptMatch, strips []string) string {
	last := len(matches) - 1
	start := last
	for start > 0 && matches[start].lineNum-matches[start-1].lineNum == 1 {
		start--
	}
	var lines []string
	for i := start; i <= last; i++ {
		line := stripNoise(matches[i].text, strips)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// scopeToLastSection extracts the content between the last two lines matching
// the section delimiter pattern. This isolates the prompt area from previous
// conversation content. Returns the full content if no pattern is set or
// fewer than two delimiters are found.
func scopeToLastSection(paneContent, sectionPattern string) string {
	if sectionPattern == "" {
		return paneContent
	}
	re, err := regexp.Compile(sectionPattern)
	if err != nil {
		return paneContent
	}
	lines := strings.Split(paneContent, "\n")
	var delimLines []int
	for i, line := range lines {
		if re.MatchString(line) {
			delimLines = append(delimLines, i)
		}
	}
	if len(delimLines) < 2 {
		return paneContent
	}
	start := delimLines[len(delimLines)-2] + 1
	end := delimLines[len(delimLines)-1]
	if start >= end {
		return paneContent
	}
	return strings.Join(lines[start:end], "\n")
}

// stripNoise removes each of the agent's StripPatterns from text and trims
// whitespace.
func stripNoise(text string, patterns []string) string {
	for _, p := range patterns {
		text = strings.ReplaceAll(text, p, "")
	}
	return strings.TrimSpace(text)
}

// sendClearSequence parses a space-separated key sequence and sends each
// token individually. Tokens with a "*N" suffix (e.g. "BSpace*200") are
// sent N times using tmux send-keys -N for efficient bulk repeats.
func sendClearSequence(paneID, clearKeys string) error {
	for _, token := range strings.Fields(clearKeys) {
		key, count := parseKeyRepeat(token)
		if count > 1 {
			if err := sendRepeatedKey(paneID, key, count); err != nil {
				return fmt.Errorf("clear key %q*%d failed: %w", key, count, err)
			}
		} else {
			if err := sendKeys(paneID, key); err != nil {
				return fmt.Errorf("clear key %q failed: %w", key, err)
			}
		}
		// Add delay after Escape to let Vim-based agents exit INSERT mode
		if key == "Escape" {
			time.Sleep(150 * time.Millisecond)
		}
	}
	return nil
}

// parseKeyRepeat splits "Key*N" into (Key, N). Returns (token, 1) if no
// repeat suffix is present or the suffix is invalid.
func parseKeyRepeat(token string) (string, int) {
	idx := strings.LastIndex(token, "*")
	if idx < 1 || idx >= len(token)-1 {
		return token, 1
	}
	n, err := strconv.Atoi(token[idx+1:])
	if err != nil || n < 1 {
		return token, 1
	}
	return token[:idx], n
}

// sendLines sends text line-by-line to a tmux pane, inserting the specified
// newline key between lines. If newlineKeys is empty, "Enter" is used as
// fallback. This is the shared text-sending logic used by agent SendText
// implementations.
func sendLines(paneID, text, newlineKeys string) error {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if err := sendKeys(paneID, line); err != nil {
			return fmt.Errorf("send line %d failed: %w", i, err)
		}
		// Insert inter-line newline (except after the last line)
		if i < len(lines)-1 {
			nlKey := newlineKeys
			if nlKey == "" {
				nlKey = "Enter"
			}
			if err := sendKeys(paneID, nlKey); err != nil {
				return fmt.Errorf("newline after line %d failed: %w", i, err)
			}
		}
	}
	return nil
}
