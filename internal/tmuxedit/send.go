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

// sleepAfterClear pauses to let the TUI drain queued keystrokes (like bulk
// backspaces) before new text is sent. Override in tests to avoid delays.
var sleepAfterClear = func() { time.Sleep(300 * time.Millisecond) }

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
