package tmuxedit

import (
	"fmt"
	"log"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

func TestRunWithConfig_HappyPath(t *testing.T) {
	deps := noSleepDeps()

	// Mock: pane resolution via tmux query
	deps.runCommand = func(name string, args ...string) ([]byte, error) {
		if name == "tmux" && args[0] == "display-message" {
			return []byte("%5"), nil
		}
		return nil, nil
	}

	// Mock: capture pane content with Aider agent detected; aider uses "> prompt" pattern
	deps.capturePane = func(paneID string) (string, error) {
		return "aider v0.50\n> fix the bug", nil
	}

	// Mock: editor popup returns modified text
	deps.openEditorPopup = func(initial, w, h string) (string, error) {
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
	deps.sendKeys = func(paneID string, keys ...string) error {
		sent = append(sent, strings.Join(keys, ","))
		return nil
	}

	cfg := appconfig.App{}
	err := deps.runWithConfig(Options{}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have sent: clear keys, then the full edited text (both lines)
	if len(sent) < 2 {
		t.Fatalf("expected at least 2 send calls (clear + text), got %d: %v", len(sent), sent)
	}

	allSent := strings.Join(sent, "|")
	if !strings.Contains(allSent, "fix the bug") {
		t.Errorf("expected 'fix the bug' in sent calls: %v", sent)
	}
	if !strings.Contains(allSent, "also refactor the module") {
		t.Errorf("expected 'also refactor the module' in sent calls: %v", sent)
	}
}

func TestRunWithConfig_ExplicitAgent(t *testing.T) {
	deps := noSleepDeps()
	deps.runCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("%1"), nil
	}
	deps.capturePane = func(string) (string, error) {
		return "some generic content\n> hello", nil
	}
	deps.openEditorPopup = func(initial, w, h string) (string, error) {
		// With cursor agent, prompt extraction uses │ pattern, so initial should be empty
		if initial != "" {
			t.Errorf("initial = %q, want empty (cursor agent doesn't match > pattern)", initial)
		}
		return "new prompt", nil
	}
	deps.sendKeys = func(string, ...string) error { return nil }

	cfg := appconfig.App{}
	err := deps.runWithConfig(Options{Agent: "cursor"}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithConfig_EditorEmpty(t *testing.T) {
	deps := noSleepDeps()
	deps.runCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("%1"), nil
	}
	deps.capturePane = func(string) (string, error) {
		return "aider v0.50\n> ", nil
	}
	deps.openEditorPopup = func(string, string, string) (string, error) {
		return "", nil // user saved empty file
	}
	deps.sendKeys = func(string, ...string) error {
		t.Fatal("sendKeys should not be called when editor returns empty")
		return nil
	}

	cfg := appconfig.App{}
	err := deps.runWithConfig(Options{}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithConfig_CustomDimensions(t *testing.T) {
	deps := noSleepDeps()
	deps.runCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("%1"), nil
	}
	deps.capturePane = func(string) (string, error) { return "", nil }
	deps.openEditorPopup = func(initial, w, h string) (string, error) {
		if w != "90%" || h != "85%" {
			t.Errorf("dimensions = %sx%s, want 90%%x85%%", w, h)
		}
		return "test", nil
	}
	deps.sendKeys = func(string, ...string) error { return nil }

	cfg := appconfig.App{
		FeatureConfig: appconfig.FeatureConfig{TmuxEditConfig: appconfig.TmuxEditConfig{
			TmuxEditPopupWidth:  "90%",
			TmuxEditPopupHeight: "85%",
		}},
	}
	err := deps.runWithConfig(Options{}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPickAgent_ExplicitName(t *testing.T) {
	agents := builtinAgents()
	got := pickAgent("cursor", "Claude Code detected", agents)
	if got.Name() != "cursor" {
		t.Errorf("pickAgent(cursor) = %q, want cursor (explicit name should win)", got.Name())
	}
}

func TestPickAgent_AutoDetect(t *testing.T) {
	agents := builtinAgents()
	got := pickAgent("", "Amp by Sourcegraph", agents)
	if got.Name() != "amp" {
		t.Errorf("pickAgent('', amp content) = %q, want amp", got.Name())
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
	var capturedArgs struct {
		ed, path, w, h string
	}
	deps := tmuxEditDeps{launchPopup: func(ed, path, w, h string) error {
		capturedArgs.ed = ed
		capturedArgs.path = path
		capturedArgs.w = w
		capturedArgs.h = h
		return nil
	}}

	err := deps.launch("vim", "/tmp/test.md", "90%", "85%")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedArgs.ed != "vim" {
		t.Errorf("ed = %q, want vim", capturedArgs.ed)
	}
	if capturedArgs.w != "90%" || capturedArgs.h != "85%" {
		t.Errorf("dimensions = %sx%s, want 90%%x85%%", capturedArgs.w, capturedArgs.h)
	}
}

func TestLaunchPopup_NoDimensions(t *testing.T) {
	var capturedArgs struct {
		w, h string
	}
	deps := tmuxEditDeps{launchPopup: func(ed, path, w, h string) error {
		capturedArgs.w = w
		capturedArgs.h = h
		return nil
	}}

	err := deps.launch("nano", "/tmp/f.md", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedArgs.w != "" || capturedArgs.h != "" {
		t.Errorf("expected empty dimensions, got %qx%q", capturedArgs.w, capturedArgs.h)
	}
}

func TestRunWithConfig_CaptureError(t *testing.T) {
	deps := tmuxEditDeps{}
	deps.runCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("%1"), nil
	}
	deps.capturePane = func(string) (string, error) {
		return "", fmt.Errorf("capture failed")
	}

	cfg := appconfig.App{}
	err := deps.runWithConfig(Options{Pane: "%1"}, cfg)
	if err == nil || !strings.Contains(err.Error(), "capture failed") {
		t.Errorf("expected capture error, got: %v", err)
	}
}

func TestRunWithConfig_EditorError(t *testing.T) {
	deps := tmuxEditDeps{}
	deps.runCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("%1"), nil
	}
	deps.capturePane = func(string) (string, error) {
		return "some content", nil
	}
	deps.openEditorPopup = func(string, string, string) (string, error) {
		return "", fmt.Errorf("editor crashed")
	}

	cfg := appconfig.App{}
	err := deps.runWithConfig(Options{Pane: "%1"}, cfg)
	if err == nil || !strings.Contains(err.Error(), "editor crashed") {
		t.Errorf("expected editor error, got: %v", err)
	}
}

func TestLogPaneLines_WithDebugLog(t *testing.T) {
	// Set up debugLog to a buffer to cover the logging branch
	var buf strings.Builder
	oldDebugLog := debugLog
	debugLog = log.New(&buf, "", 0)
	defer func() { debugLog = oldDebugLog }()

	// Content with box-drawing and arrow characters triggers logging
	content := "normal line\n│ box line\n→ arrow line\nplain"
	logPaneLines(content)

	output := buf.String()
	if !strings.Contains(output, "box line") {
		t.Errorf("expected log of box-drawing line, got: %s", output)
	}
	if !strings.Contains(output, "arrow line") {
		t.Errorf("expected log of arrow line, got: %s", output)
	}
}

func TestLogPaneLines_WithoutDebugLog(t *testing.T) {
	// When debugLog is nil, logPaneLines should not panic
	oldDebugLog := debugLog
	debugLog = nil
	defer func() { debugLog = oldDebugLog }()

	logPaneLines("│ test line\n→ arrow")
	// No panic means pass
}

func TestRunWithConfig_ClearInputError(t *testing.T) {
	deps := noSleepDeps()
	deps.runCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("%1"), nil
	}
	// Use Aider (clearFirst=true, clearKeys="C-u") so ClearInput is exercised
	deps.capturePane = func(string) (string, error) {
		return "aider v0.50\n> fix the bug", nil
	}
	deps.openEditorPopup = func(string, string, string) (string, error) {
		return "new text", nil
	}
	deps.sendKeys = func(string, ...string) error {
		return fmt.Errorf("clear input failed")
	}

	cfg := appconfig.App{}
	err := deps.runWithConfig(Options{}, cfg)
	if err == nil || !strings.Contains(err.Error(), "clear input failed") {
		t.Errorf("expected clear input error, got: %v", err)
	}
}

func TestRunWithConfig_SendTextError(t *testing.T) {
	deps := noSleepDeps()
	deps.runCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("%1"), nil
	}
	// Use generic agent (no clear) so ClearInput succeeds
	deps.capturePane = func(string) (string, error) {
		return "some unknown pane content", nil
	}
	deps.openEditorPopup = func(string, string, string) (string, error) {
		return "new text", nil
	}
	callCount := 0
	deps.sendKeys = func(string, ...string) error {
		callCount++
		// Fail on text send (generic agent has no clear)
		return fmt.Errorf("send text failed")
	}

	cfg := appconfig.App{}
	err := deps.runWithConfig(Options{}, cfg)
	if err == nil || !strings.Contains(err.Error(), "send text failed") {
		t.Errorf("expected send text error, got: %v", err)
	}
}

func TestRunWithConfig_PaneResolveError(t *testing.T) {
	deps := tmuxEditDeps{runCommand: func(string, ...string) ([]byte, error) {
		return nil, fmt.Errorf("tmux unavailable")
	}}
	t.Setenv("HEXAI_TMUX_PANE", "")

	cfg := appconfig.App{}
	err := deps.runWithConfig(Options{}, cfg)
	if err == nil {
		t.Fatal("expected error for pane resolution failure")
	}
}
