package tmuxedit

import (
	"fmt"
	"testing"
)

func TestCapturePane_Success(t *testing.T) {
	deps := tmuxEditDeps{runCommand: func(name string, args ...string) ([]byte, error) {
		if name == "tmux" && len(args) >= 3 && args[0] == "capture-pane" {
			return []byte("Claude Code v1.0\n> hello world\n"), nil
		}
		return nil, fmt.Errorf("unexpected: %s %v", name, args)
	}}
	got, err := deps.capture("%5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Claude Code v1.0\n> hello world" {
		t.Errorf("got %q, want trimmed content", got)
	}
}

func TestCapturePane_Error(t *testing.T) {
	deps := tmuxEditDeps{runCommand: func(string, ...string) ([]byte, error) {
		return nil, fmt.Errorf("pane not found")
	}}
	_, err := deps.capture("%999")
	if err == nil {
		t.Fatal("expected error for failed capture")
	}
}

func TestCapturePane_EmptyContent(t *testing.T) {
	deps := tmuxEditDeps{runCommand: func(string, ...string) ([]byte, error) {
		return []byte("\n\n"), nil
	}}
	got, err := deps.capture("%1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}
