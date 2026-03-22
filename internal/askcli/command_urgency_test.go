package askcli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestHandleUrgency_Success(t *testing.T) {
	jsonData := `[{"uuid":"uuid-2","description":"Task 2","status":"pending","priority":"M","tags":["agent"],"urgency":10.0,"depends":[]},{"uuid":"uuid-1","description":"Task 1","status":"pending","priority":"H","tags":["cli"],"urgency":15.0,"depends":[]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		io.WriteString(stdout, jsonData)
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"urgency"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("urgency code = %d, want 0", code)
	}
	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines (header + sep + 2 tasks), got %d: %q", len(lines), output)
	}
	taskLine1 := lines[2]
	taskLine2 := lines[3]
	if !strings.Contains(taskLine1, "uuid-1") {
		t.Fatalf("first task line should contain uuid-1 (urgency 15.0), got: %s", taskLine1)
	}
	if !strings.Contains(taskLine2, "uuid-2") {
		t.Fatalf("second task line should contain uuid-2 (urgency 10.0), got: %s", taskLine2)
	}
}

func TestHandleUrgency_EmptyList(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		io.WriteString(stdout, "[]")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"urgency"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("urgency code = %d, want 0 for empty list", code)
	}
}

func TestHandleUrgency_ExportFails(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 1, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"urgency"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("urgency code = %d, want 1 when export fails", code)
	}
}

func TestHandleUrgency_PassesCorrectArgs(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		capturedArgs = args
		io.WriteString(stdout, "[]")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	d.Dispatch(context.Background(), []string{"urgency"}, nil, &stdout, &stderr)
	if len(capturedArgs) != 1 || capturedArgs[0] != "export" {
		t.Fatalf("capturedArgs = %v, want [export]", capturedArgs)
	}
}
