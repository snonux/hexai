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

func TestHandleDep_AddSuccess(t *testing.T) {
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
		NextID: 2,
		Entries: []taskAliasCacheEntry{
			{UUID: "uuid-1", Alias: "0", CreatedAt: nowTaskAliasCache()},
			{UUID: "uuid-2", Alias: "1", CreatedAt: nowTaskAliasCache()},
		},
	})

	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[1] == "export" {
			switch args[0] {
			case "uuid:uuid-1":
				_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1","description":"Task","status":"pending","priority":"M","tags":[],"urgency":10,"depends":[]}]`)
			case "uuid:uuid-2":
				_, _ = io.WriteString(stdout, `[{"uuid":"uuid-2","description":"Task","status":"pending","priority":"M","tags":[],"urgency":10,"depends":[]}]`)
			}
			return 0, nil
		}
		capturedArgs = args
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"dep", "add", "uuid-1", "uuid-2"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dep add code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "ok 0") || strings.Contains(stdout.String(), "uuid-1") {
		t.Fatalf("stdout = %q, want ok + alias only", stdout.String())
	}
	// Verify uuid:<uuid> is the filter (not a modification argument).
	if len(capturedArgs) < 3 || capturedArgs[0] != "uuid:uuid-1" || capturedArgs[1] != "modify" {
		t.Fatalf("capturedArgs = %v, want [uuid:uuid-1, modify, depends:uuid-2]", capturedArgs)
	}
}

func TestHandleDep_RmSuccess(t *testing.T) {
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[1] == "export" {
			switch args[0] {
			case "uuid:uuid-1":
				_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1","description":"Task","status":"pending","priority":"M","tags":[],"urgency":10,"depends":[]}]`)
			case "uuid:uuid-2":
				_, _ = io.WriteString(stdout, `[{"uuid":"uuid-2","description":"Task","status":"pending","priority":"M","tags":[],"urgency":10,"depends":[]}]`)
			}
			return 0, nil
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"dep", "rm", "uuid-1", "uuid-2"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dep rm code = %d, want 0", code)
	}
}

func TestHandleDep_ListSuccess(t *testing.T) {
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
		NextID: 3,
		Entries: []taskAliasCacheEntry{
			{UUID: "dep-1", Alias: "1", CreatedAt: nowTaskAliasCache()},
			{UUID: "dep-2", Alias: "2", CreatedAt: nowTaskAliasCache()},
			{UUID: "uuid-1", Alias: "0", CreatedAt: nowTaskAliasCache()},
		},
	})

	jsonData := `[{"uuid":"uuid-1","description":"Task","status":"pending","priority":"M","tags":[],"urgency":10,"depends":["dep-1","dep-2"]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stdout, jsonData)
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"dep", "list", "uuid-1"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dep list code = %d, want 0", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "1") || !strings.Contains(output, "2") || strings.Contains(output, "dep-1") || strings.Contains(output, "dep-2") {
		t.Fatalf("stdout = %q, want alias deps only", output)
	}
}

func TestHandleDep_ListAssignsAliasForUnknownDependency(t *testing.T) {
	now := useIsolatedTaskAliasCache(t)

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "uuid-1", Alias: "0", CreatedAt: now},
		},
	})

	jsonData := `[{"uuid":"uuid-1","description":"Task","status":"pending","priority":"M","tags":[],"urgency":10,"depends":["dep-1"]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stdout, jsonData)
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"dep", "list", "uuid-1"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dep list code = %d, want 0", code)
	}
	if got := stdout.String(); got != "1\n" {
		t.Fatalf("stdout = %q, want assigned alias", got)
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
				if len(args) == 2 && args[1] == "export" {
					capturedArgs = args
					_, _ = io.WriteString(stdout, export)
					return 0, nil
				}
				capturedArgs = args
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

func TestHandleDep_AliasSelectors(t *testing.T) {
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
		NextID: 2,
		Entries: []taskAliasCacheEntry{
			{UUID: "uuid-1", Alias: "0", CreatedAt: nowTaskAliasCache()},
			{UUID: "uuid-2", Alias: "1", CreatedAt: nowTaskAliasCache()},
		},
	})

	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[1] == "export" {
			switch args[0] {
			case "uuid:uuid-1":
				_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1","description":"T1","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			case "uuid:uuid-2":
				_, _ = io.WriteString(stdout, `[{"uuid":"uuid-2","description":"T2","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			}
			return 0, nil
		}
		capturedArgs = args
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"dep", "add", "0", "1"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dep add code = %d stderr = %q", code, stderr.String())
	}
	if len(capturedArgs) < 3 || capturedArgs[0] != "uuid:uuid-1" || capturedArgs[2] != "depends:uuid-2" {
		t.Fatalf("capturedArgs = %v, want resolved alias UUIDs", capturedArgs)
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
	if !strings.Contains(stderr.String(), "task alias ID or UUID") {
		t.Fatalf("stderr = %q, want alias-or-UUID guidance", stderr.String())
	}
}
