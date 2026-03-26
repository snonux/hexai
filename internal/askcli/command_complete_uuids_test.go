package askcli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleCompleteUUIDs_PrintsPendingUUIDs(t *testing.T) {
	dir := t.TempDir()
	oldNow := nowTaskAliasCache
	oldRoot := taskAliasCacheRoot
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	defer func() {
		nowTaskAliasCache = oldNow
		taskAliasCacheRoot = oldRoot
	}()

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		want := []string{"status:pending", "export"}
		if strings.Join(args, " ") != strings.Join(want, " ") {
			t.Fatalf("args = %v, want %v", args, want)
		}
		_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1"},{"uuid":"uuid-2"},{"uuid":""}]`)
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.handleCompleteUUIDs(context.Background(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("handleCompleteUUIDs returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("handleCompleteUUIDs code = %d, want 0", code)
	}
	if got := stdout.String(); got != "uuid-1\nuuid-2\n" {
		t.Fatalf("stdout = %q, want UUID list", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	path, err := taskAliasCachePath()
	if err != nil {
		t.Fatalf("taskAliasCachePath: %v", err)
	}
	cache := readTaskAliasCacheForTest(t, path)
	if got := findTaskAliasEntry(t, cache, "uuid-1").Alias; got != "0" {
		t.Fatalf("uuid-1 alias = %q, want 0", got)
	}
	if got := findTaskAliasEntry(t, cache, "uuid-2").Alias; got != "1" {
		t.Fatalf("uuid-2 alias = %q, want 1", got)
	}
}

func TestHandleCompleteUUIDs_ParseError(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stdout, `not-json`)
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.handleCompleteUUIDs(context.Background(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("handleCompleteUUIDs returned error: %v", err)
	}
	if code != 1 {
		t.Fatalf("handleCompleteUUIDs code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "failed to parse task data") {
		t.Fatalf("stderr = %q, want parse error", stderr.String())
	}
}

func TestHandleCompleteUUIDs_WarnsOnInvalidAliasCache(t *testing.T) {
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	defer func() { taskAliasCacheRoot = oldRoot }()

	path, err := taskAliasCachePath()
	if err != nil {
		t.Fatalf("taskAliasCachePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("{bad"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1"}]`)
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.handleCompleteUUIDs(context.Background(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("handleCompleteUUIDs returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("handleCompleteUUIDs code = %d, want 0", code)
	}
	if got := stdout.String(); got != "uuid-1\n" {
		t.Fatalf("stdout = %q, want UUID list", got)
	}
	if !strings.Contains(stderr.String(), "failed to update task alias cache") {
		t.Fatalf("stderr = %q, want cache warning", stderr.String())
	}
}
