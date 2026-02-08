package tmuxedit

import (
	"fmt"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

func TestRunWithConfig_HappyPath(t *testing.T) {
	// Save and restore all seams
	oldCapture := capturePane
	oldSendKeys := sendKeys
	oldEditorPopup := openEditorPopup
	oldRunCmd := runCommand
	defer func() {
		capturePane = oldCapture
		sendKeys = oldSendKeys
		openEditorPopup = oldEditorPopup
		runCommand = oldRunCmd
	}()

	// Mock: pane resolution via tmux query
	runCommand = func(name string, args ...string) ([]byte, error) {
		if name == "tmux" && args[0] == "display-message" {
			return []byte("%5"), nil
		}
		return nil, nil
	}

	// Mock: capture pane content with Claude agent detected
	capturePane = func(paneID string) (string, error) {
		return "Claude Code v1.0\n> fix the bug", nil
	}

	// Mock: editor popup returns modified text
	openEditorPopup = func(initial, w, h string) (string, error) {
		if initial != "fix the bug" {
			t.Errorf("initial = %q, want 'fix the bug'", initial)
		}
		if w != "80%" || h != "80%" {
			t.Errorf("dimensions = %sx%s, want 80%%x80%%", w, h)
		}
		return "fix the bug\nalso refactor the module", nil
	}

	// Track send-keys calls
	var sent []string
	sendKeys = func(paneID string, keys ...string) error {
		sent = append(sent, strings.Join(keys, ","))
		return nil
	}

	cfg := appconfig.App{}
	err := runWithConfig(Options{}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have sent: clear (C-u), "also refactor the module"
	// (since "fix the bug" was pre-filled and kept, only the appended text is sent)
	if len(sent) < 1 {
		t.Fatalf("no send calls recorded")
	}

	// Check that the deduplication worked - only new text should be sent
	allSent := strings.Join(sent, "|")
	if !strings.Contains(allSent, "also refactor the module") {
		t.Errorf("expected 'also refactor the module' in sent calls: %v", sent)
	}
}

func TestRunWithConfig_ExplicitAgent(t *testing.T) {
	oldCapture := capturePane
	oldSendKeys := sendKeys
	oldEditorPopup := openEditorPopup
	oldRunCmd := runCommand
	defer func() {
		capturePane = oldCapture
		sendKeys = oldSendKeys
		openEditorPopup = oldEditorPopup
		runCommand = oldRunCmd
	}()

	runCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("%1"), nil
	}
	capturePane = func(string) (string, error) {
		return "some generic content\n> hello", nil
	}
	openEditorPopup = func(initial, w, h string) (string, error) {
		// With cursor agent, prompt extraction uses │ pattern, so initial should be empty
		if initial != "" {
			t.Errorf("initial = %q, want empty (cursor agent doesn't match > pattern)", initial)
		}
		return "new prompt", nil
	}
	sendKeys = func(string, ...string) error { return nil }

	cfg := appconfig.App{}
	err := runWithConfig(Options{Agent: "cursor"}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithConfig_EditorEmpty(t *testing.T) {
	oldCapture := capturePane
	oldSendKeys := sendKeys
	oldEditorPopup := openEditorPopup
	oldRunCmd := runCommand
	defer func() {
		capturePane = oldCapture
		sendKeys = oldSendKeys
		openEditorPopup = oldEditorPopup
		runCommand = oldRunCmd
	}()

	runCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("%1"), nil
	}
	capturePane = func(string) (string, error) {
		return "Claude\n> ", nil
	}
	openEditorPopup = func(string, string, string) (string, error) {
		return "", nil // user saved empty file
	}
	sendKeys = func(string, ...string) error {
		t.Fatal("sendKeys should not be called when editor returns empty")
		return nil
	}

	cfg := appconfig.App{}
	err := runWithConfig(Options{}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithConfig_CustomDimensions(t *testing.T) {
	oldCapture := capturePane
	oldSendKeys := sendKeys
	oldEditorPopup := openEditorPopup
	oldRunCmd := runCommand
	defer func() {
		capturePane = oldCapture
		sendKeys = oldSendKeys
		openEditorPopup = oldEditorPopup
		runCommand = oldRunCmd
	}()

	runCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("%1"), nil
	}
	capturePane = func(string) (string, error) { return "", nil }
	openEditorPopup = func(initial, w, h string) (string, error) {
		if w != "90%" || h != "85%" {
			t.Errorf("dimensions = %sx%s, want 90%%x85%%", w, h)
		}
		return "test", nil
	}
	sendKeys = func(string, ...string) error { return nil }

	cfg := appconfig.App{
		TmuxEditPopupWidth:  "90%",
		TmuxEditPopupHeight: "85%",
	}
	err := runWithConfig(Options{}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPickAgent_ExplicitName(t *testing.T) {
	agents := builtinAgents()
	got := pickAgent("cursor", "Claude Code detected", agents)
	if got.Name != "cursor" {
		t.Errorf("pickAgent(cursor) = %q, want cursor (explicit name should win)", got.Name)
	}
}

func TestPickAgent_AutoDetect(t *testing.T) {
	agents := builtinAgents()
	got := pickAgent("", "Amp by Sourcegraph", agents)
	if got.Name != "amp" {
		t.Errorf("pickAgent('', amp content) = %q, want amp", got.Name)
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "'simple'"},
		{"with space", "'with space'"},
		{"it's", "'it'\\''s'"},
	}
	for _, tt := range tests {
		got := shellQuote(tt.input)
		if got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLaunchPopup_CommandArgs(t *testing.T) {
	oldRunCmd := runCommand
	defer func() { runCommand = oldRunCmd }()

	var captured []string
	runCommand = func(name string, args ...string) ([]byte, error) {
		captured = append(captured, name)
		captured = append(captured, args...)
		return nil, nil
	}

	err := launchPopup("vim", "/tmp/test.md", "90%", "85%")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify command structure: tmux display-popup -E -w 90% -h 85% "vim '/tmp/test.md'"
	if captured[0] != "tmux" {
		t.Errorf("command = %q, want tmux", captured[0])
	}
	if captured[1] != "display-popup" {
		t.Errorf("args[0] = %q, want display-popup", captured[1])
	}
	if captured[2] != "-E" {
		t.Errorf("args[1] = %q, want -E", captured[2])
	}
}

func TestLaunchPopup_NoDimensions(t *testing.T) {
	oldRunCmd := runCommand
	defer func() { runCommand = oldRunCmd }()

	var captured []string
	runCommand = func(name string, args ...string) ([]byte, error) {
		captured = args
		return nil, nil
	}

	err := launchPopup("nano", "/tmp/f.md", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not include -w or -h flags
	for _, a := range captured {
		if a == "-w" || a == "-h" {
			t.Errorf("unexpected dimension flag in args: %v", captured)
		}
	}
}

func TestRunWithConfig_CaptureError(t *testing.T) {
	oldCapture := capturePane
	oldRunCmd := runCommand
	defer func() {
		capturePane = oldCapture
		runCommand = oldRunCmd
	}()

	runCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("%1"), nil
	}
	capturePane = func(string) (string, error) {
		return "", fmt.Errorf("capture failed")
	}

	cfg := appconfig.App{}
	err := runWithConfig(Options{Pane: "%1"}, cfg)
	if err == nil || !strings.Contains(err.Error(), "capture failed") {
		t.Errorf("expected capture error, got: %v", err)
	}
}

func TestRunWithConfig_EditorError(t *testing.T) {
	oldCapture := capturePane
	oldEditorPopup := openEditorPopup
	oldRunCmd := runCommand
	defer func() {
		capturePane = oldCapture
		openEditorPopup = oldEditorPopup
		runCommand = oldRunCmd
	}()

	runCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("%1"), nil
	}
	capturePane = func(string) (string, error) {
		return "some content", nil
	}
	openEditorPopup = func(string, string, string) (string, error) {
		return "", fmt.Errorf("editor crashed")
	}

	cfg := appconfig.App{}
	err := runWithConfig(Options{Pane: "%1"}, cfg)
	if err == nil || !strings.Contains(err.Error(), "editor crashed") {
		t.Errorf("expected editor error, got: %v", err)
	}
}

func TestRunWithConfig_PaneResolveError(t *testing.T) {
	oldRunCmd := runCommand
	defer func() { runCommand = oldRunCmd }()

	runCommand = func(string, ...string) ([]byte, error) {
		return nil, fmt.Errorf("tmux unavailable")
	}
	t.Setenv("HEXAI_TMUX_PANE", "")

	cfg := appconfig.App{}
	err := runWithConfig(Options{}, cfg)
	if err == nil {
		t.Fatal("expected error for pane resolution failure")
	}
}
