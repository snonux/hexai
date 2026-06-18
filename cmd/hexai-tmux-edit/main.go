// hexai-tmux-edit opens a tmux popup with $EDITOR for composing AI agent
// prompts. It captures existing prompt text from the target pane, pre-fills
// the editor, and sends the edited text back via tmux send-keys.
//
// Usage:
//
//	hexai-tmux-edit [--config <path>] [--agent <name>] [--pane <id>]
//
// Tmux keybinding (add to ~/.tmux.conf):
//
//	bind e run-shell -b "cd '#{pane_current_path}' && hexai-tmux-edit --pane '#{pane_id}'"
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/tmuxedit"
)

type app struct {
	runTmuxEdit func(tmuxedit.Options) error
}

func newApp() *app { return &app{runTmuxEdit: tmuxedit.Run} }

func main() { os.Exit(newApp().runMain(os.Args[1:], os.Stderr)) }

// runMain parses flags from args and runs the tmux edit popup. It returns
// the process exit code; flag errors return 2 (matching stdlib convention),
// runtime failures return 1.
func (a *app) runMain(args []string, stderr io.Writer) int {
	defaultPath := appconfig.DefaultConfigPath()
	fs := flag.NewFlagSet("hexai-tmux-edit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", fmt.Sprintf("path to config file (default: %s)", defaultPath))
	agent := fs.String("agent", "", "AI agent name (auto-detected if omitted)")
	pane := fs.String("pane", "", "tmux target pane ID (e.g. %5)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opts := buildOptions(*configPath, *agent, *pane)
	if err := a.runTmuxEdit(opts); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// buildOptions constructs tmuxedit.Options from the parsed flag values,
// trimming whitespace from each field.
func buildOptions(configPath, agent, pane string) tmuxedit.Options {
	return tmuxedit.Options{
		ConfigPath: strings.TrimSpace(configPath),
		Agent:      strings.TrimSpace(agent),
		Pane:       strings.TrimSpace(pane),
	}
}
