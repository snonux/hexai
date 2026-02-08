package tmuxedit

import (
	"fmt"
	"strconv"
	"strings"
	"time"
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

// deduplicateText compares the original (pre-filled) text with what the user
// returned from the editor. Returns empty string if unchanged (no-op), or
// the full edited text if anything changed. The caller is responsible for
// clearing existing pane input before sending the result, so we always return
// the complete text rather than stripping the original prefix.
func deduplicateText(original, edited string) string {
	original = strings.TrimSpace(original)
	edited = strings.TrimSpace(edited)
	if edited == "" || edited == original {
		return ""
	}
	return edited
}

// sendTextToPane sends the given text to the target pane. It optionally
// clears existing input first (using the agent's ClearKeys sequence), then
// sends text line-by-line using the agent's NewlineKeys between lines.
// ClearKeys is space-separated; tokens like "BSpace*200" repeat a key N times
// via tmux send-keys -N. Example: "End BSpace*200" moves to end then
// sends 200 backspaces to clear the entire prompt buffer.
func sendTextToPane(paneID, text string, agent AgentConfig) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	// Clear existing input using the key sequence (space-separated).
	// Each token is sent as a separate tmux send-keys call.
	// A short pause after clearing lets the TUI process all queued
	// keystrokes (e.g. 200 backspaces) before new text arrives.
	if agent.ClearFirst && agent.ClearKeys != "" {
		if err := sendClearSequence(paneID, agent.ClearKeys); err != nil {
			return err
		}
		sleepAfterClear()
	}
	// Send text line-by-line, inserting newline keys between lines
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

// sleepAfterClear pauses to let the TUI drain queued keystrokes (like bulk
// backspaces) before new text is sent. Override in tests to avoid delays.
var sleepAfterClear = func() { time.Sleep(300 * time.Millisecond) }

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
	}
	return nil
}

// sendRepeatedKey is the seam for `tmux send-keys -N <count>`. Override in
// tests. Uses -N for efficient bulk key repeats (e.g. 200 backspaces).
var sendRepeatedKey = func(paneID, key string, count int) error {
	args := []string{"send-keys", "-t", paneID, "-N", strconv.Itoa(count), key}
	_, err := runCommand("tmux", args...)
	if err != nil {
		return fmt.Errorf("send-keys -N failed: %w", err)
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
