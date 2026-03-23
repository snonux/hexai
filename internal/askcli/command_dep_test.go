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

// TestHandleDep_AcceptUUIDPrefix verifies that dep add/rm/list accept the
// "uuid:" prefix on both UUID arguments and strip it before building the filter.
func TestHandleDep_AcceptUUIDPrefix(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		wantArg0 string
	}{
		{"add with prefix", []string{"dep", "add", "uuid:uuid-1", "uuid:uuid-2"}, "uuid:uuid-1"},
		{"rm with prefix", []string{"dep", "rm", "uuid:uuid-1", "uuid:uuid-2"}, "uuid:uuid-1"},
		{"list with prefix", []string{"dep", "list", "uuid:uuid-1"}, "uuid:uuid-1"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var capturedArgs []string
			export := `[{"uuid":"uuid-1","description":"T","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`
			d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
				capturedArgs = args
				io.WriteString(stdout, export)
				return 0, nil
			}})
			var stdout, stderr bytes.Buffer
			code, _ := d.Dispatch(context.Background(), tc.args, nil, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("%s code = %d stderr = %s", tc.name, code, stderr.String())
			}
			if len(capturedArgs) == 0 || capturedArgs[0] != tc.wantArg0 {
				t.Fatalf("%s capturedArgs[0] = %q, want %q (full: %v)", tc.name, capturedArgs[0], tc.wantArg0, capturedArgs)
			}
		})
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
