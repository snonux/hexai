package main

import (
	"bytes"
	"errors"
	"strings"
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

// runMain happy path: flags parse, runTmuxEdit returns nil, exit code 0.
// We capture the resolved Options to confirm flags map onto fields correctly.
func TestRunMain_FlagsForwardedToTmuxedit(t *testing.T) {
	old := runTmuxEdit
	t.Cleanup(func() { runTmuxEdit = old })

	var got tmuxedit.Options
	runTmuxEdit = func(opts tmuxedit.Options) error {
		got = opts
		return nil
	}

	var stderr bytes.Buffer
	code := runMain([]string{"-config", "  /tmp/cfg.toml ", "-agent", "claude", "-pane", "%9"}, &stderr)
	if code != 0 {
		t.Fatalf("runMain code = %d, want 0", code)
	}
	if got.ConfigPath != "/tmp/cfg.toml" || got.Agent != "claude" || got.Pane != "%9" {
		t.Fatalf("unexpected opts: %+v", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty on success, got %q", stderr.String())
	}
}

// runMain reports tmuxedit.Run failures by writing to stderr and returning 1
// — the production exit code that the shipped binary uses.
func TestRunMain_RunErrorReturnsOne(t *testing.T) {
	old := runTmuxEdit
	t.Cleanup(func() { runTmuxEdit = old })
	runTmuxEdit = func(tmuxedit.Options) error { return errors.New("boom") }

	var stderr bytes.Buffer
	code := runMain(nil, &stderr)
	if code != 1 {
		t.Fatalf("runMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("stderr missing error: %q", stderr.String())
	}
}

// Unknown flags must yield exit 2 (the convention used by stdlib `flag` when
// ExitOnError aborts) without ever invoking runTmuxEdit.
func TestRunMain_BadFlagReturnsTwo(t *testing.T) {
	old := runTmuxEdit
	t.Cleanup(func() { runTmuxEdit = old })
	called := false
	runTmuxEdit = func(tmuxedit.Options) error {
		called = true
		return nil
	}

	var stderr bytes.Buffer
	code := runMain([]string{"--no-such-flag"}, &stderr)
	if code != 2 {
		t.Fatalf("runMain code = %d, want 2", code)
	}
	if called {
		t.Fatal("runTmuxEdit must not be called on flag-parse failure")
	}
}
