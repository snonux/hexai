package askcli

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestDispatcher_Help(t *testing.T) {
	d := NewDispatcher(nil)
	var stdout bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"help"}, nil, &stdout, io.Discard)
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
	if !strings.Contains(output, "ask all") {
		t.Fatalf("help missing all subcommand: %s", output)
	}
	if !strings.Contains(output, "ask fish") {
		t.Fatalf("help missing fish subcommand: %s", output)
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

func TestDispatcher_CompleteUUIDsSubcommand(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if strings.Join(args, " ") != "status:pending export" {
			t.Fatalf("args = %v, want pending export", args)
		}
		_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1"}]`)
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"complete-uuids"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("complete-uuids returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("complete-uuids code = %d, want 0", code)
	}
	if got := stdout.String(); got != "uuid-1\n" {
		t.Fatalf("stdout = %q, want UUID list", got)
	}
}

func TestDispatcher_LongHelp(t *testing.T) {
	d := NewDispatcher(nil)
	var stdout bytes.Buffer
	d.Dispatch(context.Background(), []string{"help"}, nil, &stdout, io.Discard)
	output := stdout.String()
	for _, sub := range []string{"add", "list", "all", "ready", "info", "annotate", "start", "stop", "done", "priority", "tag", "dep", "urgency", "modify", "denotate", "delete", "fish"} {
		if !strings.Contains(output, "ask "+sub) {
			t.Errorf("help missing subcommand: ask %s", sub)
		}
	}
}

func TestDispatcher_FishSubcommand(t *testing.T) {
	d := NewDispatcher(nil)
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"fish"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("fish returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("fish code = %d, want 0", code)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if got := stdout.String(); got != FishCompletionFor(exe) {
		t.Fatalf("fish output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, FishCompletionFor(exe))
	}
	if stderr.Len() != 0 {
		t.Fatalf("fish wrote unexpected stderr: %q", stderr.String())
	}
}

func TestDispatcher_FishSubcommandRejectsExtraArgs(t *testing.T) {
	d := NewDispatcher(nil)
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"fish", "extra"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("fish extra args returned error: %v", err)
	}
	if code != 1 {
		t.Fatalf("fish extra args code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("fish extra args wrote unexpected stdout: %q", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, "usage: ask fish") {
		t.Fatalf("fish extra args stderr = %q, want usage", got)
	}
}

func TestDispatcher_AllSubcommandsReachExecutor(t *testing.T) {
	subcommands := []string{}
	subcommandArgs := map[string][]string{
		"delete":   {"delete", "test-uuid"},
		"denotate": {"denotate", "test-uuid", "text"},
		"modify":   {"modify", "test-uuid", "priority:H"},
		"annotate": {"annotate", "test-uuid", "note"},
		"start":    {"start", "test-uuid"},
		"stop":     {"stop", "test-uuid"},
		"done":     {"done", "test-uuid"},
		"priority": {"priority", "test-uuid", "H"},
		"tag":      {"tag", "test-uuid", "+cli"},
		"dep":      {"dep", "list", "test-uuid"},
		"list":     {"list"},
		"all":      {"all"},
		"ready":    {"ready"},
		"urgency":  {"urgency"},
		"info":     {"info", "test-uuid"},
		"add":      {"add", "new task description"},
	}
	for _, sub := range subcommands {
		var stdout, stderr bytes.Buffer
		calls := 0
		d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout_, stderr_ io.Writer) (int, error) {
			calls++
			if args[0] == "export" || args[0] == "uuid" {
				io.WriteString(stdout_, `[{"uuid":"test-uuid","description":"Test","status":"pending","priority":"M","tags":[],"urgency":10,"depends":[]}]`)
			}
			if args[0] == "add" {
				io.WriteString(stdout_, "Created task 123.\nUUID: test-uuid-abc")
			}
			return 0, nil
		}})
		args := []string{sub}
		if extra, ok := subcommandArgs[sub]; ok {
			args = extra
		}
		code, _ := d.Dispatch(context.Background(), args, nil, &stdout, &stderr)
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
