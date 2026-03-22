package askcli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestHandleDelete_Success(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"delete", "test-uuid-123"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("delete code = %d, want 0", code)
	}
	if err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "ok") || !strings.Contains(stdout.String(), "test-uuid-123") {
		t.Fatalf("stdout = %q, want ok + uuid", stdout.String())
	}
	if stderr.Len() > 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestHandleDelete_NumericID(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for numeric ID")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"delete", "123"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("delete code = %d, want 1 for numeric ID", code)
	}
	if err != nil {
		t.Fatalf("delete returned unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "use UUID") {
		t.Fatalf("stderr = %q, want 'use UUID' message", stderr.String())
	}
}

func TestHandleDelete_MissingUUID(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for missing UUID")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"delete"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("delete code = %d, want 1 for missing UUID", code)
	}
	if err != nil {
		t.Fatalf("delete returned unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "requires a UUID") {
		t.Fatalf("stderr = %q, want 'requires a UUID' message", stderr.String())
	}
}

func TestHandleDelete_CommandFails(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 1, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"delete", "test-uuid-456"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("delete code = %d, want 1 for failed command", code)
	}
	if stdout.Len() > 0 {
		t.Fatalf("stdout should be empty on failure, got %q", stdout.String())
	}
}

func TestHandleDelete_PassesCorrectArgs(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		capturedArgs = args
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	d.Dispatch(context.Background(), []string{"delete", "my-uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if len(capturedArgs) != 2 || capturedArgs[0] != "uuid:my-uuid" || capturedArgs[1] != "delete" {
		t.Fatalf("capturedArgs = %v, want [uuid:my-uuid, delete]", capturedArgs)
	}
}
