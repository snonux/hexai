package main

import (
	"errors"
	"testing"

	"codeberg.org/snonux/hexai/internal/tmuxedit"
)

func TestBuildOptions_AllEmpty(t *testing.T) {
	opts := buildOptions("", "", "")
	if opts.ConfigPath != "" || opts.Agent != "" || opts.Pane != "" {
		t.Fatalf("expected all empty, got %+v", opts)
	}
}

func TestBuildOptions_TrimsWhitespace(t *testing.T) {
	opts := buildOptions("  /tmp/cfg.toml  ", "  claude  ", "  %5  ")
	if opts.ConfigPath != "/tmp/cfg.toml" {
		t.Fatalf("expected trimmed config path, got %q", opts.ConfigPath)
	}
	if opts.Agent != "claude" {
		t.Fatalf("expected trimmed agent, got %q", opts.Agent)
	}
	if opts.Pane != "%5" {
		t.Fatalf("expected trimmed pane, got %q", opts.Pane)
	}
}

func TestRunTmuxEdit_Success(t *testing.T) {
	old := runTmuxEdit
	t.Cleanup(func() { runTmuxEdit = old })

	var gotOpts tmuxedit.Options
	runTmuxEdit = func(opts tmuxedit.Options) error {
		gotOpts = opts
		return nil
	}

	opts := buildOptions("/tmp/cfg.toml", "cursor", "%3")
	if err := runTmuxEdit(opts); err != nil {
		t.Fatalf("runTmuxEdit: %v", err)
	}
	if gotOpts.ConfigPath != "/tmp/cfg.toml" || gotOpts.Agent != "cursor" || gotOpts.Pane != "%3" {
		t.Fatalf("unexpected opts: %+v", gotOpts)
	}
}

func TestRunTmuxEdit_Error(t *testing.T) {
	old := runTmuxEdit
	t.Cleanup(func() { runTmuxEdit = old })

	wantErr := errors.New("tmux not found")
	runTmuxEdit = func(_ tmuxedit.Options) error { return wantErr }

	if err := runTmuxEdit(tmuxedit.Options{}); !errors.Is(err, wantErr) {
		t.Fatalf("expected error, got: %v", err)
	}
}
