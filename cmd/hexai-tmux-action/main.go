package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/hexaiaction"
)

func main() {
	infile := flag.String("infile", "", "Read input from this file instead of stdin")
	outfile := flag.String("outfile", "", "Write output to this file instead of stdout")
	uiChild := flag.Bool("ui-child", false, "INTERNAL: run interactive UI and write to -outfile atomically")
	defaultPath := defaultConfigPath()
	configPath := flag.String("config", "", fmt.Sprintf("path to config file (default: %s)", defaultPath))
	tmuxTarget := flag.String("tmux-target", "", "tmux split target (advanced)")
	tmuxSplit := flag.String("tmux-split", "v", "tmux split orientation: v or h")
	tmuxPercent := flag.Int("tmux-percent", 33, "tmux split size percentage (1-100)")
	flag.Parse()

	opts := hexaiaction.Options{
		Infile: *infile, Outfile: *outfile,
		UIChild: *uiChild, TmuxTarget: *tmuxTarget, TmuxSplit: *tmuxSplit, TmuxPercent: *tmuxPercent,
	}
	ctx := context.Background()
	if path := strings.TrimSpace(*configPath); path != "" {
		ctx = hexaiaction.WithConfigPath(ctx, path)
	}
	if err := hexaiaction.RunCommand(ctx, opts, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func defaultConfigPath() string {
	path, err := appconfig.ConfigPath()
	if err != nil {
		return "$XDG_CONFIG_HOME/hexai/config.toml"
	}
	return path
}
