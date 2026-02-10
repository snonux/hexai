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
	"os"
	"strings"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/tmuxedit"
)

func main() {
	defaultPath := defaultConfigPath()
	configPath := flag.String("config", "", fmt.Sprintf("path to config file (default: %s)", defaultPath))
	agent := flag.String("agent", "", "AI agent name (auto-detected if omitted)")
	pane := flag.String("pane", "", "tmux target pane ID (e.g. %%5)")
	flag.Parse()

	opts := tmuxedit.Options{
		ConfigPath: strings.TrimSpace(*configPath),
		Agent:      strings.TrimSpace(*agent),
		Pane:       strings.TrimSpace(*pane),
	}
	if err := tmuxedit.Run(opts); err != nil {
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
