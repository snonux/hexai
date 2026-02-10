package tmuxedit

import (
	"fmt"
	"log"
	"os"
	"os/exec"
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

// launchPopup is the seam for running `tmux display-popup` with the editor.
// The -E flag makes the popup close when the editor exits. The -d flag sets
// the working directory for the popup. Uses .Run() (not .Output()) so the
// popup blocks until the user closes the editor.
var launchPopup = func(ed, path, width, height string) error {
	args := []string{"display-popup", "-E"}

	// Get current working directory to pass to the popup
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		args = append(args, "-d", cwd)
	}

	if width != "" {
		args = append(args, "-w", width)
	}
	if height != "" {
		args = append(args, "-h", height)
	}
	args = append(args, ed+" "+shellQuote(path))
	return exec.Command("tmux", args...).Run()
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

// debugLog is the debug logger. Set to a real logger via initDebugLog().
var debugLog *log.Logger

// initDebugLog creates a debug log file at /tmp/hexai-tmux-edit.log.
func initDebugLog() {
	f, err := os.OpenFile("/tmp/hexai-tmux-edit.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	debugLog = log.New(f, "", log.LstdFlags|log.Lmicroseconds)
}

func dbg(format string, args ...any) {
	if debugLog != nil {
		debugLog.Printf(format, args...)
	}
}

// runWithConfig executes the edit workflow using the provided config.
// It resolves the agent (by name or auto-detect), extracts the current
// prompt, opens the editor popup, then clears and sends the result.
func runWithConfig(opts Options, cfg appconfig.App) error {
	initDebugLog()
	dbg("=== hexai-tmux-edit start ===")
	dbg("opts: pane=%q agent=%q config=%q", opts.Pane, opts.Agent, opts.ConfigPath)

	paneID, err := resolveTargetPane(opts.Pane)
	if err != nil {
		dbg("resolveTargetPane error: %v", err)
		return err
	}
	dbg("resolved pane: %q", paneID)

	content, err := capturePane(paneID)
	if err != nil {
		dbg("capturePane error: %v", err)
		return err
	}
	dbg("captured %d bytes from pane", len(content))
	logPaneLines(content)

	agents := resolveAgents(cfg.TmuxEditAgents)
	agent := pickAgent(opts.Agent, content, agents)
	dbg("agent: name=%q", agent.Name())

	original := agent.ExtractPrompt(content)
	dbg("extractPrompt result: %q", original)

	popupW, popupH := popupDimensions(cfg)
	dbg("opening editor popup: w=%s h=%s initial=%q", popupW, popupH, original)

	edited, err := openEditorPopup(original, popupW, popupH)
	if err != nil {
		dbg("openEditorPopup error: %v", err)
		return err
	}
	dbg("editor returned: %q", edited)

	text := deduplicateText(original, edited)
	dbg("deduplicateText result: %q", text)
	if text == "" {
		dbg("nothing to send, exiting")
		return nil
	}

	dbg("clearing and sending to pane %q: %q", paneID, text)
	if err := agent.ClearInput(paneID); err != nil {
		dbg("ClearInput error: %v", err)
		return err
	}
	if err := agent.SendText(paneID, text); err != nil {
		dbg("SendText error: %v", err)
		return err
	}
	dbg("=== done ===")
	return nil
}

// logPaneLines logs lines containing box-drawing or arrow characters for
// debugging prompt detection.
func logPaneLines(content string) {
	for i, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "│") || strings.Contains(line, "→") {
			dbg("  pane line %d: %q", i, line)
		}
	}
}

// popupDimensions returns the popup width and height from config, defaulting
// to "80%" for both if not set.
func popupDimensions(cfg appconfig.App) (string, string) {
	w := cfg.TmuxEditPopupWidth
	if w == "" {
		w = "80%"
	}
	h := cfg.TmuxEditPopupHeight
	if h == "" {
		h = "80%"
	}
	return w, h
}

// pickAgent selects an agent by explicit name or auto-detection.
func pickAgent(name, content string, agents []Agent) Agent {
	if name != "" {
		return findAgentByName(name, agents)
	}
	return detectAgent(content, agents)
}
