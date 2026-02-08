package tmuxedit

import (
	"regexp"
	"strings"
)

// claudeAgent handles Claude Code's ❯ prompt between ──── horizontal rules.
// Claude Code runs in actual vim mode, so clearing uses vim commands.
// Wrapped text appears as indented continuation lines without ❯.
type claudeAgent struct{ baseAgent }

// newClaudeAgent returns a claudeAgent with the default configuration.
// SectionPattern scopes extraction to the last ─── delimited area, avoiding
// false positives from ❯ in previous messages.
func newClaudeAgent() *claudeAgent {
	return &claudeAgent{baseAgent{
		name:          "claude",
		displayName:   "Claude Code",
		detectPattern: `(❯|(?i)claude code|(?i)anthropic)`,
		sectionPat:    `^─{5,}`,
		promptPat:     `(?m)❯\s*(.+)$`,
		clearFirst:    true,
		clearKeys:     "Escape gg C-v G d i",
		newlineKeys:   "S-Enter",
		submitKeys:    "Enter",
	}}
}

// ExtractPrompt extracts the prompt text from the last section between ─────
// rules. Within the scoped section, all non-empty lines are collected:
// ❯-prefixed lines have the prefix stripped, and indented continuation lines
// (wrapped text without ❯) are included as-is after trimming.
func (c *claudeAgent) ExtractPrompt(paneContent string) string {
	if c.promptPat == "" {
		return ""
	}
	re, err := regexp.Compile(c.promptPat)
	if err != nil {
		return ""
	}
	// Scope to the last section between ───── delimiters
	content := scopeToLastSection(paneContent, c.sectionPat)
	// Collect ❯-prefixed lines and their continuation lines (indented
	// wrapped text without ❯). Only include non-❯ lines that directly
	// follow a ❯-matched line to avoid picking up unrelated content.
	paneLines := strings.Split(content, "\n")
	var lines []string
	inPrompt := false
	for _, line := range paneLines {
		m := re.FindStringSubmatch(line)
		if len(m) >= 2 {
			// ❯-prefixed line: use the captured text
			cleaned := stripNoise(m[1], c.stripPatterns)
			if cleaned != "" {
				lines = append(lines, cleaned)
			}
			inPrompt = true
		} else if inPrompt {
			// Non-❯ line after a prompt: include indented continuation text
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				lines = append(lines, trimmed)
			} else {
				// Empty line breaks the continuation
				inPrompt = false
			}
		}
	}
	return strings.Join(lines, "\n")
}

// ClearInput sends vim commands to clear Claude Code's input:
// Escape to ensure normal mode, gg to go to top, C-v G d to visual-block
// select all and delete, then i to re-enter insert mode.
func (c *claudeAgent) ClearInput(paneID string) error {
	if !c.clearFirst || c.clearKeys == "" {
		return nil
	}
	if err := sendClearSequence(paneID, c.clearKeys); err != nil {
		return err
	}
	sleepAfterClear()
	return nil
}
