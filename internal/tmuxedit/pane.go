package tmuxedit

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// runCommand is the seam for exec.Command().Output(). Override in tests.
var runCommand = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// resolveTargetPane determines which tmux pane to target using a fallback
// chain: explicit flag > HEXAI_TMUX_PANE env var > tmux query for active pane.
// Returns the pane ID (e.g. "%5") or an error.
func resolveTargetPane(flagPane string) (string, error) {
	// 1. Explicit --pane flag
	if p := strings.TrimSpace(flagPane); p != "" {
		return p, nil
	}
	// 2. Environment variable
	if p := strings.TrimSpace(os.Getenv("HEXAI_TMUX_PANE")); p != "" {
		return p, nil
	}
	// 3. Query tmux for the active pane in the current window
	return queryActivePane()
}

// queryActivePane asks tmux for the active pane ID using display-message.
func queryActivePane() (string, error) {
	out, err := runCommand("tmux", "display-message", "-p", "#{pane_id}")
	if err != nil {
		return "", fmt.Errorf("cannot determine tmux pane: %w", err)
	}
	pane := strings.TrimSpace(string(out))
	if pane == "" {
		return "", fmt.Errorf("tmux returned empty pane ID")
	}
	return pane, nil
}
