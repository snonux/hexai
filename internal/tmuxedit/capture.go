package tmuxedit

import (
	"fmt"
	"strings"
)

// capturePane retrieves the visible content of a tmux pane via
// `tmux capture-pane -p -t <paneID>`. The -p flag prints to stdout
// instead of to a paste buffer.
var capturePane = func(paneID string) (string, error) {
	out, err := runCommand("tmux", "capture-pane", "-p", "-t", paneID)
	if err != nil {
		return "", fmt.Errorf("capture-pane failed for %s: %w", paneID, err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}
