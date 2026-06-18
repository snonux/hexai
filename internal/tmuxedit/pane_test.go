package tmuxedit

import (
	"fmt"
	"testing"
)

func TestResolveTargetPane_FlagWins(t *testing.T) {
	deps := tmuxEditDeps{runCommand: func(string, ...string) ([]byte, error) {
		return []byte("%99"), nil
	}}
	t.Setenv("HEXAI_TMUX_PANE", "%10")
	got, err := deps.resolveTargetPane("%5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "%5" {
		t.Errorf("got %q, want %%5 (flag should win)", got)
	}
}

func TestResolveTargetPane_EnvFallback(t *testing.T) {
	deps := tmuxEditDeps{runCommand: func(string, ...string) ([]byte, error) {
		return []byte("%99"), nil
	}}
	t.Setenv("HEXAI_TMUX_PANE", "%10")
	got, err := deps.resolveTargetPane("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "%10" {
		t.Errorf("got %q, want %%10 (env fallback)", got)
	}
}

func TestResolveTargetPane_TmuxQuery(t *testing.T) {
	deps := tmuxEditDeps{runCommand: func(name string, args ...string) ([]byte, error) {
		if name == "tmux" && len(args) > 0 && args[0] == "display-message" {
			return []byte("%42\n"), nil
		}
		return nil, fmt.Errorf("unexpected command: %s", name)
	}}
	t.Setenv("HEXAI_TMUX_PANE", "")
	got, err := deps.resolveTargetPane("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "%42" {
		t.Errorf("got %q, want %%42 (tmux query)", got)
	}
}

func TestResolveTargetPane_TmuxError(t *testing.T) {
	deps := tmuxEditDeps{runCommand: func(string, ...string) ([]byte, error) {
		return nil, fmt.Errorf("tmux not available")
	}}
	t.Setenv("HEXAI_TMUX_PANE", "")
	_, err := deps.resolveTargetPane("")
	if err == nil {
		t.Fatal("expected error when tmux fails")
	}
}

func TestResolveTargetPane_TmuxEmptyOutput(t *testing.T) {
	deps := tmuxEditDeps{runCommand: func(string, ...string) ([]byte, error) {
		return []byte("  \n"), nil
	}}
	t.Setenv("HEXAI_TMUX_PANE", "")
	_, err := deps.resolveTargetPane("")
	if err == nil {
		t.Fatal("expected error for empty tmux output")
	}
}
