package tmuxedit

import (
	"regexp"
)

// cursorAgent handles Cursor's distinctive box-drawing │ → prompt │ UI.
// Cursor uses a text field (not vim), so clearing is done with End + bulk
// backspace. Multi-line prompts are entered with Shift-Enter within the box.
type cursorAgent struct{ baseAgent }

// newCursorAgent returns a cursorAgent with the default configuration.
// Detect by the box structure or "/ commands" footer. Checked first because
// cursor panes often show model names like "Claude 4.5 Sonnet".
func newCursorAgent() *cursorAgent {
	return &cursorAgent{baseAgent{
		name:          "cursor",
		displayName:   "Cursor",
		detectPattern: `(│\s*→|/ commands · @ files)`,
		promptPat:     `(?m)│\s*→?\s*(.+?)\s*│\s*$`,
		stripPatterns: []string{"INSERT", "Add a follow-up", "ctrl+c to stop"},
		clearFirst:    true,
		clearKeys:     "End BSpace*200",
		newlineKeys:   "S-Enter",
		submitKeys:    "Enter",
	}}
}

// ExtractPrompt extracts the prompt text from the last contiguous │...│ block
// in the pane. This avoids picking up earlier command-review or dialog boxes
// that also use box-drawing characters.
func (c *cursorAgent) ExtractPrompt(paneContent string) string {
	if c.promptPat == "" {
		return ""
	}
	re, err := regexp.Compile(c.promptPat)
	if err != nil {
		return ""
	}
	allMatches := matchPromptLines(re, paneContent)
	if len(allMatches) == 0 {
		return ""
	}
	return joinLastContiguousBlock(allMatches, c.stripPatterns)
}

// ClearInput sends End + 200 backspaces to clear Cursor's text field.
// Cursor's input is a standard text field, not vim.
func (c *cursorAgent) ClearInput(paneID string) error {
	if !c.clearFirst || c.clearKeys == "" {
		return nil
	}
	if err := sendClearSequence(paneID, c.clearKeys); err != nil {
		return err
	}
	sleepAfterClear()
	return nil
}
