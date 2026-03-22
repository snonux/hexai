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
		if args[0] == "uuid" {
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
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		io.WriteString(stdout, "Created task 123.\nUUID: abc-123-def")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"add", "New task description"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("add code = %d, want 0", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "abc-123-def") {
		t.Fatalf("output missing UUID: %s", output)
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

func TestHandleAdd_MultipleWords(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		capturedArgs = args
		io.WriteString(stdout, "Created task 123.\nUUID: xyz-789")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	d.Dispatch(context.Background(), []string{"add", "Multi", "word", "description"}, nil, &stdout, &stderr)
	if len(capturedArgs) < 2 || capturedArgs[0] != "add" {
		t.Fatalf("capturedArgs = %v, want [add, Multi word description]", capturedArgs)
	}
}
