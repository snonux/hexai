package askcli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestHandleDep_AddSuccess(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		capturedArgs = args
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"dep", "add", "uuid-1", "uuid-2"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dep add code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "ok") || !strings.Contains(stdout.String(), "uuid-1") {
		t.Fatalf("stdout = %q, want ok + uuid", stdout.String())
	}
	// Verify uuid:<uuid> is the filter (not a modification argument).
	if len(capturedArgs) < 3 || capturedArgs[0] != "uuid:uuid-1" || capturedArgs[1] != "modify" {
		t.Fatalf("capturedArgs = %v, want [uuid:uuid-1, modify, depends:uuid-2]", capturedArgs)
	}
}

func TestHandleDep_RmSuccess(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"dep", "rm", "uuid-1", "uuid-2"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dep rm code = %d, want 0", code)
	}
}

func TestHandleDep_ListSuccess(t *testing.T) {
	jsonData := `[{"uuid":"uuid-1","description":"Task","status":"pending","priority":"M","tags":[],"urgency":10,"depends":["dep-1","dep-2"]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		io.WriteString(stdout, jsonData)
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"dep", "list", "uuid-1"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dep list code = %d, want 0", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "dep-1") || !strings.Contains(output, "dep-2") {
		t.Fatalf("stdout = %q, want deps", output)
	}
}

func TestHandleDep_UnknownOp(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"dep", "unknown", "uuid-1", "uuid-2"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("dep unknown code = %d, want 1", code)
	}
}

func TestHandleDep_NumericUUID(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"dep", "add", "123", "uuid-2"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("dep add code = %d, want 1 for numeric UUID", code)
	}
}
