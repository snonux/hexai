package tmux

import (
    "os"
    "os/exec"
    "strconv"
    "strings"
)

// Available reports whether tmux is available and we appear to be in a tmux session.
func Available() bool { return HasBinary() && InSession() }

// HasBinary reports whether the tmux binary is on PATH.
var lookPath = exec.LookPath
var command = exec.Command

func HasBinary() bool { _, err := lookPath("tmux"); return err == nil }

// InSession reports whether we seem to be running inside a tmux session.
func InSession() bool { return strings.TrimSpace(os.Getenv("TMUX")) != "" }

// SplitOpts controls how a new pane is created for running a command.
type SplitOpts struct {
	Target   string // optional pane target, e.g. ":."
	Vertical bool   // true => split vertically (-v); false => horizontally (-h)
	Percent  int    // 1..100; 0 means use tmux default
}

// SplitRun splits the current tmux window and runs argv in the new pane.
// It returns once tmux has launched the child process.
func SplitRun(opts SplitOpts, argv []string) error {
	if len(argv) == 0 {
		return nil
	}
	args := []string{"split-window"}
	if opts.Vertical {
		args = append(args, "-v")
	} else {
		args = append(args, "-h")
	}
	if opts.Percent > 0 && opts.Percent <= 100 {
		args = append(args, "-p", strconv.Itoa(opts.Percent))
	}
	if strings.TrimSpace(opts.Target) != "" {
		args = append(args, "-t", opts.Target)
	}
	// tmux takes a single command string. Use a conservative shell join.
	cmdStr := shellJoin(argv)
	args = append(args, cmdStr)
    c := command("tmux", args...)
    return c.Run()
}

// shellJoin quotes argv elements for safe use in a single shell command string.
// It avoids interpretation by wrapping in single quotes and escaping embedded single quotes.
func shellJoin(argv []string) string {
	out := make([]string, 0, len(argv))
	for _, a := range argv {
		if a == "" {
			out = append(out, "''")
			continue
		}
		if isSafeBare(a) {
			out = append(out, a)
			continue
		}
		// single-quote wrapping with escaped single quotes
		// ' => '\'' (close, escaped quote, reopen)
		esc := strings.ReplaceAll(a, "'", "'\\''")
		out = append(out, "'"+esc+"'")
	}
	return strings.Join(out, " ")
}

// isSafeBare returns true if a contains only safe characters for bare words.
func isSafeBare(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.' || b == '/' || b == ':' {
			continue
		}
		return false
	}
	return true
}
