package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/hexaiaction"
)

// actionRunner is the dependency that performs the tmux action. Production
// code uses hexaiaction.RunCommand; tests inject a stub to avoid launching the
// real tmux popup.
type actionRunner func(context.Context, hexaiaction.Options, io.Reader, io.Writer, io.Writer) error

func main() { os.Exit(newApp().runMain(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

// runMain parses command-line flags from args, builds actionOptions, and
// delegates to run. It returns the process exit code: 2 for flag-parse
// errors (matching stdlib `flag.ExitOnError`), 1 for runtime failures, 0 on
// success. Splitting the body out of main keeps it testable without
// touching package-level flag state. It is a method on app so tests can inject
// a stub runCommand.
func (a *app) runMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
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
	if err := a.run(opts, stdin, stdout, stderr); err != nil {
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

// app wires the injected dependencies for the command. runCommand defaults to
// hexaiaction.RunCommand in production and is replaced by tests.
type app struct {
	runCommand actionRunner
}

// newApp returns an app with the production action runner installed.
func newApp() *app { return &app{runCommand: hexaiaction.RunCommand} }

// run builds the hexaiaction.Options and context, then delegates to the
// injected runCommand dependency.
func (a *app) run(opts actionOptions, stdin io.Reader, stdout, stderr io.Writer) error {
	haOpts := hexaiaction.Options{
		Infile: opts.infile, Outfile: opts.outfile,
		UIChild: opts.uiChild, TmuxTarget: opts.tmuxTarget,
		TmuxPopupWidth: opts.tmuxPopupWidth, TmuxPopupHeight: opts.tmuxPopupHeight,
	}
	ctx := context.Background()
	if path := strings.TrimSpace(opts.configPath); path != "" {
		ctx = hexaiaction.WithConfigPath(ctx, path)
	}
	return a.runCommand(ctx, haOpts, stdin, stdout, stderr)
}
