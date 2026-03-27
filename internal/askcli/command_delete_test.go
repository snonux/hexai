package askcli

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleDelete_Success(t *testing.T) {
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "test-uuid-123", Alias: "0", CreatedAt: nowTaskAliasCache()},
		},
	})

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "uuid:test-uuid-123" && args[1] == "export" {
			io.WriteString(stdout, `[{"uuid":"test-uuid-123","description":"Task","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		}
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
	if !strings.Contains(stdout.String(), "ok 0") || strings.Contains(stdout.String(), "test-uuid-123") {
		t.Fatalf("stdout = %q, want ok + alias only", stdout.String())
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
	if !strings.Contains(stderr.String(), "task alias ID or UUID") {
		t.Fatalf("stderr = %q, want alias-or-UUID message", stderr.String())
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
	if !strings.Contains(stderr.String(), "requires an ID or UUID") {
		t.Fatalf("stderr = %q, want 'requires an ID or UUID' message", stderr.String())
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
		if len(args) == 2 && args[1] == "export" {
			io.WriteString(stdout, `[{"uuid":"my-uuid","description":"Task","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		}
		capturedArgs = args
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	d.Dispatch(context.Background(), []string{"delete", "my-uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if len(capturedArgs) != 2 || capturedArgs[0] != "uuid:my-uuid" || capturedArgs[1] != "delete" {
		t.Fatalf("capturedArgs = %v, want [uuid:my-uuid, delete]", capturedArgs)
	}
}

func TestHandleDelete_AliasSelector(t *testing.T) {
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "test-uuid-123", Alias: "0", CreatedAt: nowTaskAliasCache()},
		},
	})

	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "uuid:test-uuid-123" && args[1] == "export" {
			io.WriteString(stdout, `[{"uuid":"test-uuid-123","description":"Task","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		}
		capturedArgs = args
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"delete", "0"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("delete code = %d stderr = %q", code, stderr.String())
	}
	if len(capturedArgs) != 2 || capturedArgs[0] != "uuid:test-uuid-123" || capturedArgs[1] != "delete" {
		t.Fatalf("capturedArgs = %v, want [uuid:test-uuid-123 delete]", capturedArgs)
	}
}
