package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/snonux/hexai/internal/hexaiaction"
)

func TestRun_DelegatesToRunCommand(t *testing.T) {
	var gotOpts hexaiaction.Options
	a := &app{runCommand: func(_ context.Context, opts hexaiaction.Options, _ io.Reader, _, _ io.Writer) error {
		gotOpts = opts
		return nil
	}}

	opts := actionOptions{
		infile: "in.txt", outfile: "out.txt",
		tmuxPopupWidth: "90%", tmuxPopupHeight: "70%",
	}
	if err := a.run(opts, nil, nil, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotOpts.Infile != "in.txt" || gotOpts.Outfile != "out.txt" {
		t.Fatalf("unexpected opts: %+v", gotOpts)
	}
	if gotOpts.TmuxPopupWidth != "90%" || gotOpts.TmuxPopupHeight != "70%" {
		t.Fatalf("unexpected tmux opts: %+v", gotOpts)
	}
}

func TestRun_WithConfigPath(t *testing.T) {
	a := &app{runCommand: func(_ context.Context, _ hexaiaction.Options, _ io.Reader, _, _ io.Writer) error {
		return nil
	}}

	opts := actionOptions{configPath: "  /tmp/test.toml  "}
	if err := a.run(opts, nil, nil, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestRun_Error(t *testing.T) {
	wantErr := errors.New("action failed")
	a := &app{runCommand: func(_ context.Context, _ hexaiaction.Options, _ io.Reader, _, _ io.Writer) error {
		return wantErr
	}}

	if err := a.run(actionOptions{}, nil, nil, nil); !errors.Is(err, wantErr) {
		t.Fatalf("expected error, got: %v", err)
	}
}

// runMain happy path: every flag is forwarded into hexaiaction.Options and
// the stub returns 0. The captured Options confirm the field-by-field
// mapping that main relies on.
func TestRunMain_FlagsForwardedToHexaiaction(t *testing.T) {
	var got hexaiaction.Options
	a := &app{runCommand: func(_ context.Context, opts hexaiaction.Options, _ io.Reader, _, _ io.Writer) error {
		got = opts
		return nil
	}}

	args := []string{
		"-infile", "in.txt",
		"-outfile", "out.txt",
		"-tmux-target", "%2",
		"-tmux-popup-width", "70%",
		"-tmux-popup-height", "40%",
		"-ui-child",
	}
	var stderr bytes.Buffer
	code := a.runMain(args, nil, &bytes.Buffer{}, &stderr)
	if code != 0 {
		t.Fatalf("runMain code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if got.Infile != "in.txt" || got.Outfile != "out.txt" {
		t.Fatalf("infile/outfile mismatch: %+v", got)
	}
	if got.TmuxTarget != "%2" || got.TmuxPopupWidth != "70%" || got.TmuxPopupHeight != "40%" {
		t.Fatalf("tmux flags mismatch: %+v", got)
	}
	if !got.UIChild {
		t.Fatal("expected UIChild=true")
	}
}

// On runCommand failure, runMain returns 1 (the production exit code) and
// writes the error message to stderr so users see what went wrong.
func TestRunMain_RuntimeErrorReturnsOne(t *testing.T) {
	a := &app{runCommand: func(context.Context, hexaiaction.Options, io.Reader, io.Writer, io.Writer) error {
		return errors.New("action exploded")
	}}

	var stderr bytes.Buffer
	code := a.runMain(nil, nil, &bytes.Buffer{}, &stderr)
	if code != 1 {
		t.Fatalf("runMain code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "action exploded") {
		t.Fatalf("stderr missing error: %q", stderr.String())
	}
}

// Bad flag must yield exit 2 without ever invoking runCommand.
func TestRunMain_BadFlagReturnsTwo(t *testing.T) {
	called := false
	a := &app{runCommand: func(context.Context, hexaiaction.Options, io.Reader, io.Writer, io.Writer) error {
		called = true
		return nil
	}}
	var stderr bytes.Buffer
	code := a.runMain([]string{"--bogus"}, nil, &bytes.Buffer{}, &stderr)
	if code != 2 {
		t.Fatalf("runMain code = %d, want 2", code)
	}
	if called {
		t.Fatal("runCommand must not be called on flag-parse failure")
	}
}
