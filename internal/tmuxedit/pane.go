package tmuxedit

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type tmuxEditDeps struct {
	runCommand      func(string, ...string) ([]byte, error)
	capturePane     func(string) (string, error)
	openEditorPopup func(string, string, string) (string, error)
	sendKeys        func(string, ...string) error
	sendRepeatedKey func(string, string, int) error
	sleepAfterClear func()
	launchPopup     func(string, string, string, string) error
}

func (d tmuxEditDeps) command(name string, args ...string) ([]byte, error) {
	if d.runCommand != nil {
		return d.runCommand(name, args...)
	}
	return exec.Command(name, args...).Output()
}

// resolveTargetPane determines which tmux pane to target using a fallback
// chain: explicit flag > HEXAI_TMUX_PANE env var > tmux query for active pane.
// Returns the pane ID (e.g. "%5") or an error.
func resolveTargetPane(flagPane string) (string, error) {
	return tmuxEditDeps{}.resolveTargetPane(flagPane)
}

func (d tmuxEditDeps) resolveTargetPane(flagPane string) (string, error) {
	// 1. Explicit --pane flag
	if p := strings.TrimSpace(flagPane); p != "" {
		return p, nil
	}
	// 2. Environment variable
	if p := strings.TrimSpace(os.Getenv("HEXAI_TMUX_PANE")); p != "" {
		return p, nil
	}
	// 3. Query tmux for the active pane in the current window
	return d.queryActivePane()
}

// queryActivePane asks tmux for the active pane ID using display-message.
func queryActivePane() (string, error) {
	return tmuxEditDeps{}.queryActivePane()
}

func (d tmuxEditDeps) queryActivePane() (string, error) {
	out, err := d.command("tmux", "display-message", "-p", "#{pane_id}")
	if err != nil {
		return "", fmt.Errorf("cannot determine tmux pane: %w", err)
	}
	pane := strings.TrimSpace(string(out))
	if pane == "" {
		return "", fmt.Errorf("tmux returned empty pane ID")
	}
	return pane, nil
}
