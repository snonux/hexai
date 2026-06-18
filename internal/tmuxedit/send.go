package tmuxedit

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func sendKeys(paneID string, keys ...string) error {
	return tmuxEditDeps{}.send(paneID, keys...)
}

func (d tmuxEditDeps) send(paneID string, keys ...string) error {
	if d.sendKeys != nil {
		return d.sendKeys(paneID, keys...)
	}
	args := append([]string{"send-keys", "-t", paneID}, keys...)
	_, err := d.command("tmux", args...)
	if err != nil {
		return fmt.Errorf("send-keys failed: %w", err)
	}
	return nil
}

func sendRepeatedKey(paneID, key string, count int) error {
	return tmuxEditDeps{}.sendRepeated(paneID, key, count)
}

// sendRepeated uses `tmux send-keys -N <count>` for efficient bulk key repeats
// (e.g. 200 backspaces).
func (d tmuxEditDeps) sendRepeated(paneID, key string, count int) error {
	if d.sendRepeatedKey != nil {
		return d.sendRepeatedKey(paneID, key, count)
	}
	args := []string{"send-keys", "-t", paneID, "-N", strconv.Itoa(count), key}
	_, err := d.command("tmux", args...)
	if err != nil {
		return fmt.Errorf("send-keys -N failed: %w", err)
	}
	return nil
}

func sleepAfterClear() { tmuxEditDeps{}.sleep() }

func (d tmuxEditDeps) sleep() {
	if d.sleepAfterClear != nil {
		d.sleepAfterClear()
		return
	}
	time.Sleep(300 * time.Millisecond)
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
