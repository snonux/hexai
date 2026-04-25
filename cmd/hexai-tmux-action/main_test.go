package main

import (
	"context"
	"errors"
	"io"
	"testing"

	"codeberg.org/snonux/hexai/internal/hexaiaction"
)

func TestRun_DelegatesToRunCommand(t *testing.T) {
	old := runCommand
	t.Cleanup(func() { runCommand = old })

	var gotOpts hexaiaction.Options
	runCommand = func(_ context.Context, opts hexaiaction.Options, _ io.Reader, _, _ io.Writer) error {
		gotOpts = opts
		return nil
	}

	opts := actionOptions{
		infile: "in.txt", outfile: "out.txt",
		tmuxPopupWidth: "90%", tmuxPopupHeight: "70%",
	}
	if err := run(opts, nil, nil, nil); err != nil {
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
	old := runCommand
	t.Cleanup(func() { runCommand = old })

	runCommand = func(_ context.Context, _ hexaiaction.Options, _ io.Reader, _, _ io.Writer) error {
		return nil
	}

	opts := actionOptions{configPath: "  /tmp/test.toml  "}
	if err := run(opts, nil, nil, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestRun_Error(t *testing.T) {
	old := runCommand
	t.Cleanup(func() { runCommand = old })

	wantErr := errors.New("action failed")
	runCommand = func(_ context.Context, _ hexaiaction.Options, _ io.Reader, _, _ io.Writer) error {
		return wantErr
	}

	if err := run(actionOptions{}, nil, nil, nil); !errors.Is(err, wantErr) {
		t.Fatalf("expected error, got: %v", err)
	}
}
