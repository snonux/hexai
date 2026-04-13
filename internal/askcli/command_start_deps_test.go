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

func TestHandleStart_BlockedWhenDependencyNotCompleted(t *testing.T) {
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	fixedNow := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return fixedNow }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 2,
		Entries: []taskAliasCacheEntry{
			{UUID: "main-uuid", Alias: "0", CreatedAt: fixedNow},
			{UUID: "dep-uuid", Alias: "1", CreatedAt: fixedNow},
		},
	})

	var startCalls int
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		switch {
		case len(args) == 2 && args[0] == "uuid:main-uuid" && args[1] == "export":
			_, _ = io.WriteString(stdout, `[{"uuid":"main-uuid","description":"Main","status":"pending","priority":"M","tags":[],"urgency":0,"depends":["dep-uuid"]}]`)
			return 0, nil
		case len(args) == 2 && args[0] == "uuid:dep-uuid" && args[1] == "export":
			_, _ = io.WriteString(stdout, `[{"uuid":"dep-uuid","description":"Dep","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		case len(args) == 2 && args[0] == "uuid:main-uuid" && args[1] == "start":
			startCalls++
			return 0, nil
		default:
			t.Fatalf("unexpected runner args: %v", args)
			return 1, nil
		}
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"start", "main-uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("start code = %d, want 1", code)
	}
	if startCalls != 0 {
		t.Fatalf("task start ran %d times, want 0", startCalls)
	}
	if !strings.Contains(stderr.String(), "cannot start until all dependencies are completed") {
		t.Fatalf("stderr = %q, want dependency gate message", stderr.String())
	}
	if !strings.Contains(stderr.String(), "1 (pending)") {
		t.Fatalf("stderr should name incomplete dependency with alias and status: %q", stderr.String())
	}
}

func TestHandleStart_AllowedWhenDependencyCompleted(t *testing.T) {
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	fixedNow := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return fixedNow }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 2,
		Entries: []taskAliasCacheEntry{
			{UUID: "main-uuid", Alias: "0", CreatedAt: fixedNow},
			{UUID: "dep-uuid", Alias: "1", CreatedAt: fixedNow},
		},
	})

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		switch {
		case len(args) == 2 && args[0] == "uuid:main-uuid" && args[1] == "export":
			_, _ = io.WriteString(stdout, `[{"uuid":"main-uuid","description":"Main","status":"pending","priority":"M","tags":[],"urgency":0,"depends":["dep-uuid"]}]`)
			return 0, nil
		case len(args) == 2 && args[0] == "uuid:dep-uuid" && args[1] == "export":
			_, _ = io.WriteString(stdout, `[{"uuid":"dep-uuid","description":"Dep","status":"completed","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		case len(args) == 2 && args[0] == "uuid:main-uuid" && args[1] == "start":
			return 0, nil
		default:
			t.Fatalf("unexpected runner args: %v", args)
			return 1, nil
		}
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"start", "main-uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("start code = %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok 0") {
		t.Fatalf("stdout = %q, want ok + alias", stdout.String())
	}
}

func TestHandleStart_CompletedStatusIsCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	fixedNow := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return fixedNow }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 2,
		Entries: []taskAliasCacheEntry{
			{UUID: "main-uuid", Alias: "0", CreatedAt: fixedNow},
			{UUID: "dep-uuid", Alias: "1", CreatedAt: fixedNow},
		},
	})

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		switch {
		case len(args) == 2 && args[0] == "uuid:main-uuid" && args[1] == "export":
			_, _ = io.WriteString(stdout, `[{"uuid":"main-uuid","description":"Main","status":"pending","priority":"M","tags":[],"urgency":0,"depends":["dep-uuid"]}]`)
			return 0, nil
		case len(args) == 2 && args[0] == "uuid:dep-uuid" && args[1] == "export":
			_, _ = io.WriteString(stdout, `[{"uuid":"dep-uuid","description":"Dep","status":"COMPLETED","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		case len(args) == 2 && args[0] == "uuid:main-uuid" && args[1] == "start":
			return 0, nil
		default:
			t.Fatalf("unexpected runner args: %v", args)
			return 1, nil
		}
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"start", "main-uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("start code = %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok 0") {
		t.Fatalf("stdout = %q, want ok + alias", stdout.String())
	}
}
