package tmuxedit

import (
	"fmt"
	"strings"
)

func capturePane(paneID string) (string, error) {
	return tmuxEditDeps{}.capture(paneID)
}

// capture retrieves the visible content of a tmux pane via `tmux capture-pane
// -p -t <paneID>`. The -p flag prints to stdout instead of to a paste buffer.
func (d tmuxEditDeps) capture(paneID string) (string, error) {
	if d.capturePane != nil {
		return d.capturePane(paneID)
	}
	out, err := d.command("tmux", "capture-pane", "-p", "-t", paneID)
	if err != nil {
		return "", fmt.Errorf("capture-pane failed for %s: %w", paneID, err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}
