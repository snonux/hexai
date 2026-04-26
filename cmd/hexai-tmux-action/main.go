package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/hexaiaction"
)

// runCommand is the seam for testing: override in tests to avoid launching
// the real tmux action.
var runCommand = hexaiaction.RunCommand

func main() { os.Exit(runMain(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

// runMain parses command-line flags from args, builds actionOptions, and
// delegates to run. It returns the process exit code: 2 for flag-parse
// errors (matching stdlib `flag.ExitOnError`), 1 for runtime failures, 0 on
// success. Splitting the body out of main keeps it testable without
// touching package-level flag state.
func runMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hexai-tmux-action", flag.ContinueOnError)
	fs.SetOutput(stderr)
	infile := fs.String("infile", "", "Read input from this file instead of stdin")
	outfile := fs.String("outfile", "", "Write output to this file instead of stdout")
	uiChild := fs.Bool("ui-child", false, "INTERNAL: run interactive UI and write to -outfile atomically")
	defaultPath := appconfig.DefaultConfigPath()
	configPath := fs.String("config", "", fmt.Sprintf("path to config file (default: %s)", defaultPath))
	tmuxTarget := fs.String("tmux-target", "", "tmux popup target pane (advanced)")
	tmuxPopupWidth := fs.String("tmux-popup-width", "60%", "tmux popup width, e.g. 60% or 120")
	tmuxPopupHeight := fs.String("tmux-popup-height", "50%", "tmux popup height, e.g. 50% or 30")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opts := actionOptions{
		infile: *infile, outfile: *outfile,
		uiChild: *uiChild, configPath: *configPath,
		tmuxTarget: *tmuxTarget, tmuxPopupWidth: *tmuxPopupWidth, tmuxPopupHeight: *tmuxPopupHeight,
	}
	if err := run(opts, stdin, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// actionOptions holds the parsed command-line flags for hexai-tmux-action.
type actionOptions struct {
	infile          string
	outfile         string
	uiChild         bool
	configPath      string
	tmuxTarget      string
	tmuxPopupWidth  string
	tmuxPopupHeight string
}

// run builds the hexaiaction.Options and context, then delegates to runCommand.
func run(opts actionOptions, stdin io.Reader, stdout, stderr io.Writer) error {
	haOpts := hexaiaction.Options{
		Infile: opts.infile, Outfile: opts.outfile,
		UIChild: opts.uiChild, TmuxTarget: opts.tmuxTarget,
		TmuxPopupWidth: opts.tmuxPopupWidth, TmuxPopupHeight: opts.tmuxPopupHeight,
	}
	ctx := context.Background()
	if path := strings.TrimSpace(opts.configPath); path != "" {
		ctx = hexaiaction.WithConfigPath(ctx, path)
	}
	return runCommand(ctx, haOpts, stdin, stdout, stderr)
}
