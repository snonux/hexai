package tmuxedit

import (
	"fmt"
	"log"
	"os"
	"strings"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/editor"
	"codeberg.org/snonux/hexai/internal/tmux"
)

// Options holds the parsed command-line flags for hexai-tmux-edit.
type Options struct {
	ConfigPath string // --config flag
	Agent      string // --agent flag (explicit agent name, or auto-detect)
	Pane       string // --pane flag (target pane ID)
}

// openEditorPopup is the seam for opening an editor in a tmux popup.
// It creates a temp file, opens it in a tmux popup with the user's editor,
// waits for completion, and returns the edited content. Override in tests.
var openEditorPopup = func(initial, popupW, popupH string) (string, error) {
	ed, err := editor.Resolve()
	if err != nil {
		return "", err
	}
	// Create a temp file with the initial content
	f, err := os.CreateTemp("", "hexai-tmux-edit-*.md")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	path := f.Name()
	defer func() { _ = os.Remove(path) }()

	if initial != "" {
		if _, err := f.WriteString(initial); err != nil {
			_ = f.Close()
			return "", fmt.Errorf("write initial content: %w", err)
		}
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}

	// Build the tmux display-popup command to launch the editor
	if err := launchPopup(ed, path, popupW, popupH); err != nil {
		return "", fmt.Errorf("popup editor: %w", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read edited file: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

// launchPopup runs `tmux display-popup` with the editor command.
// The -E flag makes the popup close when the editor exits.
func launchPopup(ed, path, width, height string) error {
	args := []string{"display-popup", "-E"}
	if width != "" {
		args = append(args, "-w", width)
	}
	if height != "" {
		args = append(args, "-h", height)
	}
	args = append(args, ed+" "+shellQuote(path))
	_, err := runCommand("tmux", args...)
	return err
}

// shellQuote wraps a path in single quotes for safe shell use.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// Run is the main orchestrator for hexai-tmux-edit. It:
// 1. Checks tmux availability
// 2. Resolves the target pane
// 3. Captures pane content
// 4. Detects or selects the agent
// 5. Extracts the current prompt
// 6. Opens the editor in a popup
// 7. Deduplicates and sends edited text back
func Run(opts Options) error {
	if !tmux.Available() {
		return fmt.Errorf("tmux is not available (not in a tmux session)")
	}
	cfg := loadConfig(opts.ConfigPath)
	return runWithConfig(opts, cfg)
}

// loadConfig loads the application config, extracting tmux_edit settings.
func loadConfig(configPath string) appconfig.App {
	logger := log.New(os.Stderr, "[hexai-tmux-edit] ", log.LstdFlags)
	lopts := appconfig.LoadOptions{ConfigPath: configPath}
	return appconfig.LoadWithOptions(logger, lopts)
}

// runWithConfig executes the edit workflow using the provided config.
func runWithConfig(opts Options, cfg appconfig.App) error {
	paneID, err := resolveTargetPane(opts.Pane)
	if err != nil {
		return err
	}
	content, err := capturePane(paneID)
	if err != nil {
		return err
	}
	agents := resolveAgents(cfg.TmuxEditAgents)
	agent := pickAgent(opts.Agent, content, agents)

	// Extract current prompt text from pane content
	original := extractPrompt(content, agent)

	// Determine popup dimensions from config (with defaults)
	popupW := cfg.TmuxEditPopupWidth
	if popupW == "" {
		popupW = "80%"
	}
	popupH := cfg.TmuxEditPopupHeight
	if popupH == "" {
		popupH = "80%"
	}

	edited, err := openEditorPopup(original, popupW, popupH)
	if err != nil {
		return err
	}

	// Deduplicate: remove the pre-filled text if the user didn't change it
	text := deduplicateText(original, edited)
	if text == "" {
		return nil // nothing to send
	}

	return sendTextToPane(paneID, text, agent)
}

// pickAgent selects an agent by explicit name or auto-detection.
func pickAgent(name, content string, agents []AgentConfig) AgentConfig {
	if name != "" {
		return findAgentByName(name, agents)
	}
	return detectAgent(content, agents)
}
