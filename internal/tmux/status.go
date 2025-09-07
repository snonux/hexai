package tmux

import (
    "os"
    "os/exec"
    "strings"
)

// Enabled reports whether tmux status updates are enabled via env (default: on).
func Enabled() bool {
    v := strings.TrimSpace(os.Getenv("HEXAI_TMUX_STATUS"))
    if v == "" { return true }
    v = strings.ToLower(v)
    return v == "1" || v == "true" || v == "yes" || v == "on"
}

// SetUserOption sets a global tmux user option like @hexai_status to value.
func SetUserOption(key, value string) error {
    if !Enabled() || !HasBinary() || !InSession() { return nil }
    k := strings.TrimPrefix(strings.TrimSpace(key), "@")
    if k == "" { return nil }
    // Use set-option -g so it appears for all windows
    return exec.Command("tmux", "set-option", "-g", "@"+k, value).Run()
}

// SetStatus is a convenience for setting @hexai_status.
func SetStatus(value string) error { return SetUserOption("hexai_status", value) }

