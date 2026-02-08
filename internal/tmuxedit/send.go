package tmuxedit

import (
	"fmt"
	"strings"
)

// sendKeys is the seam for `tmux send-keys`. Override in tests.
var sendKeys = func(paneID string, keys ...string) error {
	args := append([]string{"send-keys", "-t", paneID}, keys...)
	_, err := runCommand("tmux", args...)
	if err != nil {
		return fmt.Errorf("send-keys failed: %w", err)
	}
	return nil
}

// deduplicateText removes the original (pre-filled) text from the edited
// result. If the user kept the original and appended, only the new text is
// returned. If the user rewrote everything, the full new text is returned.
func deduplicateText(original, edited string) string {
	original = strings.TrimSpace(original)
	edited = strings.TrimSpace(edited)
	if edited == "" {
		return ""
	}
	if original == "" {
		return edited
	}
	// If the edited text starts with the original, return only the appended part
	if strings.HasPrefix(edited, original) {
		appended := strings.TrimSpace(edited[len(original):])
		if appended != "" {
			return appended
		}
		// User didn't change anything; return empty to signal no-op
		return ""
	}
	// User rewrote the prompt; return the full new text
	return edited
}

// sendTextToPane sends the given text to the target pane. It optionally
// clears existing input first (using the agent's ClearKeys), then sends
// text line-by-line using the agent's NewlineKeys between lines.
func sendTextToPane(paneID, text string, agent AgentConfig) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	// Clear existing input if configured
	if agent.ClearFirst && agent.ClearKeys != "" {
		if err := sendKeys(paneID, agent.ClearKeys); err != nil {
			return fmt.Errorf("clear failed: %w", err)
		}
	}
	// Send text line by line, inserting newline keys between lines
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if err := sendKeys(paneID, line); err != nil {
			return fmt.Errorf("send line %d failed: %w", i, err)
		}
		// Insert inter-line newline (except after the last line)
		if i < len(lines)-1 {
			nlKey := agent.NewlineKeys
			if nlKey == "" {
				nlKey = "Enter" // fallback for agents without shift-enter
			}
			if err := sendKeys(paneID, nlKey); err != nil {
				return fmt.Errorf("newline after line %d failed: %w", i, err)
			}
		}
	}
	return nil
}
