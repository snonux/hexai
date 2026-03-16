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

func main() {
	infile := flag.String("infile", "", "Read input from this file instead of stdin")
	outfile := flag.String("outfile", "", "Write output to this file instead of stdout")
	uiChild := flag.Bool("ui-child", false, "INTERNAL: run interactive UI and write to -outfile atomically")
	defaultPath := appconfig.DefaultConfigPath()
	configPath := flag.String("config", "", fmt.Sprintf("path to config file (default: %s)", defaultPath))
	tmuxTarget := flag.String("tmux-target", "", "tmux split target (advanced)")
	tmuxSplit := flag.String("tmux-split", "v", "tmux split orientation: v or h")
	tmuxPercent := flag.Int("tmux-percent", 33, "tmux split size percentage (1-100)")
	flag.Parse()

	opts := actionOptions{
		infile: *infile, outfile: *outfile,
		uiChild: *uiChild, configPath: *configPath,
		tmuxTarget: *tmuxTarget, tmuxSplit: *tmuxSplit, tmuxPercent: *tmuxPercent,
	}
	if err := run(opts, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// actionOptions holds the parsed command-line flags for hexai-tmux-action.
type actionOptions struct {
	infile      string
	outfile     string
	uiChild     bool
	configPath  string
	tmuxTarget  string
	tmuxSplit   string
	tmuxPercent int
}

// run builds the hexaiaction.Options and context, then delegates to runCommand.
func run(opts actionOptions, stdin io.Reader, stdout, stderr io.Writer) error {
	haOpts := hexaiaction.Options{
		Infile: opts.infile, Outfile: opts.outfile,
		UIChild: opts.uiChild, TmuxTarget: opts.tmuxTarget,
		TmuxSplit: opts.tmuxSplit, TmuxPercent: opts.tmuxPercent,
	}
	ctx := context.Background()
	if path := strings.TrimSpace(opts.configPath); path != "" {
		ctx = hexaiaction.WithConfigPath(ctx, path)
	}
	return runCommand(ctx, haOpts, stdin, stdout, stderr)
}
