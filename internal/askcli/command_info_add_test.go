package askcli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestHandleInfo_Success(t *testing.T) {
	jsonData := `[{"uuid":"test-uuid","description":"Test task","status":"pending","priority":"H","tags":["cli","agent"],"urgency":15.0,"depends":["dep-1"],"annotations":[{"description":"Note 1","entry":"2026-03-22T10:00:00Z"}]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		// args[0] is "uuid:<uuid>" (the filter); emit data for any export call
		if len(args) > 0 && strings.HasPrefix(args[0], "uuid:") {
			io.WriteString(stdout, jsonData)
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"info", "test-uuid"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("info code = %d, want 0", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "test-uuid") {
		t.Fatalf("output missing UUID: %s", output)
	}
	if !strings.Contains(output, "H") {
		t.Fatalf("output missing priority: %s", output)
	}
}

func TestHandleInfo_NumericID(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"info", "123"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("info code = %d, want 1 for numeric ID", code)
	}
}

func TestHandleInfo_MissingUUID(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"info"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("info code = %d, want 1 for missing UUID", code)
	}
}

func TestHandleAdd_Success(t *testing.T) {
	// With rc.verbose=new-uuid, task add outputs "Created task <uuid>." directly.
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		io.WriteString(stdout, "Created task abc-123-def.")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"add", "New task description"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("add code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "abc-123-def") {
		t.Fatalf("output missing UUID: %s", stdout.String())
	}
}

func TestHandleAdd_MissingDescription(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"add"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("add code = %d, want 1 for missing description", code)
	}
}

func makeAddRunner(onAdd func(args []string, stdout io.Writer)) *spyRunner {
	return &spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		onAdd(args, stdout)
		return 0, nil
	}}
}

func TestHandleAdd_MultipleWords(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(makeAddRunner(func(args []string, stdout io.Writer) {
		capturedArgs = args
		io.WriteString(stdout, "Created task test-uuid.")
	}))
	var stdout, stderr bytes.Buffer
	d.Dispatch(context.Background(), []string{"add", "Multi", "word", "description"}, nil, &stdout, &stderr)
	// args[0]="add", args[1]="rc.verbose=new-uuid", then description
	if len(capturedArgs) < 3 || capturedArgs[0] != "add" || capturedArgs[1] != "rc.verbose=new-uuid" {
		t.Fatalf("capturedArgs = %v, want [add, rc.verbose=new-uuid, ...]", capturedArgs)
	}
}

func TestHandleAdd_WithPriority(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(makeAddRunner(func(args []string, stdout io.Writer) {
		capturedArgs = args
		io.WriteString(stdout, "Created task test-uuid.")
	}))
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"add", "priority:H", "Fix critical bug"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("add code = %d, want 0", code)
	}
	// args: [add, rc.verbose=new-uuid, priority:H, Fix critical bug]
	if len(capturedArgs) < 4 {
		t.Fatalf("capturedArgs = %v, want at least 4 elements", capturedArgs)
	}
	if capturedArgs[2] != "priority:H" {
		t.Errorf("capturedArgs[2] = %s, want priority:H", capturedArgs[2])
	}
	if capturedArgs[len(capturedArgs)-1] != "Fix critical bug" {
		t.Errorf("last arg = %s, want 'Fix critical bug'", capturedArgs[len(capturedArgs)-1])
	}
}

func TestHandleAdd_WithTag(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(makeAddRunner(func(args []string, stdout io.Writer) {
		capturedArgs = args
		io.WriteString(stdout, "Created task test-uuid.")
	}))
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"add", "+cli", "New feature"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("add code = %d, want 0", code)
	}
	// args: [add, rc.verbose=new-uuid, +cli, New feature]
	if capturedArgs[2] != "+cli" {
		t.Errorf("capturedArgs[2] = %s, want +cli", capturedArgs[2])
	}
}

func TestHandleAdd_WithPriorityAndTag(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(makeAddRunner(func(args []string, stdout io.Writer) {
		capturedArgs = args
		io.WriteString(stdout, "Created task test-uuid.")
	}))
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"add", "priority:H", "+cli", "Complex task"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("add code = %d, want 0", code)
	}
	// args: [add, rc.verbose=new-uuid, priority:H, +cli, Complex task]
	if capturedArgs[2] != "priority:H" || capturedArgs[3] != "+cli" {
		t.Errorf("capturedArgs = %v, want [add, rc.verbose=new-uuid, priority:H, +cli, Complex task]", capturedArgs)
	}
}

func TestExtractUUIDFromAddOutput(t *testing.T) {
	if uuid := extractUUIDFromAddOutput("Created task abc-123-def."); uuid != "abc-123-def" {
		t.Fatalf("got %q, want abc-123-def", uuid)
	}
	if uuid := extractUUIDFromAddOutput("Created task abc-123-def.\nsome other line"); uuid != "abc-123-def" {
		t.Fatalf("got %q, want abc-123-def", uuid)
	}
	if uuid := extractUUIDFromAddOutput("no match here"); uuid != "" {
		t.Fatalf("got %q, want empty", uuid)
	}
}

func TestParseAddArgs(t *testing.T) {
	mods, desc := parseAddArgs([]string{"priority:H", "+cli", "Fix bug"})
	if desc != "Fix bug" || len(mods) != 2 {
		t.Fatalf("parseAddArgs([\"priority:H\", \"+cli\", \"Fix bug\"]) = mods=%v, desc=%q, want mods=[priority:H, +cli], desc=\"Fix bug\"", mods, desc)
	}

	mods, desc = parseAddArgs([]string{"Multi", "word", "description"})
	if desc != "Multi word description" || len(mods) != 0 {
		t.Fatalf("parseAddArgs([\"Multi\", \"word\", \"description\"]) = mods=%v, desc=%q, want mods=[], desc=\"Multi word description\"", mods, desc)
	}

	mods, desc = parseAddArgs([]string{"-deprecated", "Old task"})
	if desc != "Old task" || len(mods) != 1 || mods[0] != "-deprecated" {
		t.Fatalf("parseAddArgs([\"-deprecated\", \"Old task\"]) = mods=%v, desc=%q", mods, desc)
	}
}
