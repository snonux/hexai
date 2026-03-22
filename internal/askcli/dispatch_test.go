package askcli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestDispatcher_Help(t *testing.T) {
	d := NewDispatcher(nil)
	var stdout bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("help exit code = %d, want 0", code)
	}
	if err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "ask - task management CLI") {
		t.Fatalf("help missing title: %s", output)
	}
	if !strings.Contains(output, "ask list") {
		t.Fatalf("help missing list subcommand: %s", output)
	}
	if !strings.Contains(output, "Filters:") {
		t.Fatalf("help missing Filters section: %s", output)
	}
}

func TestDispatcher_UnknownSubcommand(t *testing.T) {
	d := NewDispatcher(nil)
	var stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"foobar"}, nil, io.Discard, &stderr)
	if code != 1 {
		t.Fatalf("unknown subcommand exit code = %d, want 1", code)
	}
	if err != nil {
		t.Fatalf("unknown subcommand returned unexpected error: %v", err)
	}
	output := stderr.String()
	if !strings.Contains(output, "unknown subcommand") {
		t.Fatalf("unknown subcommand output missing: %s", output)
	}
}

func TestDispatcher_LongHelp(t *testing.T) {
	d := NewDispatcher(nil)
	var stdout bytes.Buffer
	d.Dispatch(context.Background(), []string{}, nil, &stdout, io.Discard)
	output := stdout.String()
	for _, sub := range []string{"add", "list", "info", "annotate", "start", "stop", "done", "priority", "tag", "dep", "urgency", "modify", "denotate", "delete", "export"} {
		if !strings.Contains(output, "ask "+sub) {
			t.Errorf("help missing subcommand: ask %s", sub)
		}
	}
}

func TestDispatcher_AllSubcommandsReachExecutor(t *testing.T) {
	subcommands := []string{"add", "list", "info", "annotate", "start", "stop", "done", "priority", "tag", "dep", "urgency", "modify", "denotate", "delete", "export"}
	for _, sub := range subcommands {
		var stdout, stderr bytes.Buffer
		calls := 0
		d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout_, stderr_ io.Writer) (int, error) {
			calls++
			return 0, nil
		}})
		code, _ := d.Dispatch(context.Background(), []string{sub}, nil, &stdout, &stderr)
		if code != 0 {
			t.Errorf("subcommand %q code = %d, want 0", sub, code)
		}
	}
}

type spyRunner struct {
	runFn func(context.Context, []string, io.Reader, io.Writer, io.Writer) (int, error)
}

func (s *spyRunner) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return s.runFn(ctx, args, stdin, stdout, stderr)
}
