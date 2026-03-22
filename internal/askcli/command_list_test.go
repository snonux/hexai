package askcli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestHandleList_Success(t *testing.T) {
	jsonData := `[{"uuid":"uuid-1","description":"Task 1","status":"pending","priority":"H","tags":["cli"],"urgency":15.0,"depends":[]},{"uuid":"uuid-2","description":"Task 2","status":"completed","priority":"M","tags":["agent"],"urgency":10.0,"depends":[]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		for _, arg := range args {
			if arg == "export" {
				io.WriteString(stdout, jsonData)
				return 0, nil
			}
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"list"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("list code = %d, want 0", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "uuid-1") || !strings.Contains(output, "uuid-2") {
		t.Fatalf("output missing UUIDs: %s", output)
	}
}

func TestHandleList_SortedByPriority(t *testing.T) {
	jsonData := `[{"uuid":"uuid-2","description":"Task 2","status":"pending","priority":"M","tags":[],"urgency":10.0,"depends":[]},{"uuid":"uuid-1","description":"Task 1","status":"pending","priority":"H","tags":[],"urgency":5.0,"depends":[]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		for _, arg := range args {
			if arg == "export" {
				io.WriteString(stdout, jsonData)
				return 0, nil
			}
		}
		return 0, nil
	}})
	var stdout bytes.Buffer
	d.Dispatch(context.Background(), []string{"list"}, nil, &stdout, &bytes.Buffer{})
	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	taskLine1 := lines[2]
	if !strings.Contains(taskLine1, "uuid-1") {
		t.Fatalf("first task should be H priority (uuid-1), got: %s", taskLine1)
	}
}

func TestHandleList_EmptyList(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		for _, arg := range args {
			if arg == "export" {
				io.WriteString(stdout, "[]")
				return 0, nil
			}
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"list"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("list code = %d, want 0 for empty list", code)
	}
}

func TestHandleList_PassesFilters(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		capturedArgs = args
		io.WriteString(stdout, "[]")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	d.Dispatch(context.Background(), []string{"list", "+READY", "limit:5", "sort:priority-"}, nil, &stdout, &stderr)
	if len(capturedArgs) < 2 {
		t.Fatalf("expected export args, got %v", capturedArgs)
	}
	hasExport := false
	for _, arg := range capturedArgs {
		if arg == "export" {
			hasExport = true
			break
		}
	}
	if !hasExport {
		t.Fatalf("expected export in args, got %v", capturedArgs)
	}
}
