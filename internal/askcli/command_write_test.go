package askcli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func useIsolatedTaskAliasCache(t *testing.T) time.Time {
	t.Helper()

	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	fixedNow := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return fixedNow }
	t.Cleanup(func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	})
	return fixedNow
}

func TestHandleStart_AliasSelector(t *testing.T) {
	now := useIsolatedTaskAliasCache(t)

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "test-uuid", Alias: "0", CreatedAt: now},
		},
	})

	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "uuid:test-uuid" && args[1] == "export" {
			io.WriteString(stdout, `[{"uuid":"test-uuid","description":"Task","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		}
		capturedArgs = args
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"start", "0"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("start code = %d stderr = %q", code, stderr.String())
	}
	if len(capturedArgs) != 2 || capturedArgs[0] != "uuid:test-uuid" || capturedArgs[1] != "start" {
		t.Fatalf("capturedArgs = %v, want [uuid:test-uuid start]", capturedArgs)
	}
}

func TestHandleDenotate_Success(t *testing.T) {
	now := useIsolatedTaskAliasCache(t)

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "test-uuid", Alias: "0", CreatedAt: now},
		},
	})

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "uuid:test-uuid" && args[1] == "export" {
			io.WriteString(stdout, `[{"uuid":"test-uuid","description":"Task","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"denotate", "test-uuid", "old annotation"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("denotate code = %d, want 0", code)
	}
	if err != nil {
		t.Fatalf("denotate returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "ok 0") || strings.Contains(stdout.String(), "test-uuid") {
		t.Fatalf("stdout = %q, want ok + alias only", stdout.String())
	}
}

func TestHandleDenotate_NumericID(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for numeric ID")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"denotate", "123", "text"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("denotate code = %d, want 1 for numeric ID", code)
	}
}

func TestHandleDenotate_MissingArgs(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for missing args")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"denotate", "uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("denotate code = %d, want 1 for missing args", code)
	}
	if !strings.Contains(stderr.String(), "requires an ID or UUID") {
		t.Fatalf("stderr = %q, want ID-or-UUID message", stderr.String())
	}
}

func TestHandleModify_Success(t *testing.T) {
	now := useIsolatedTaskAliasCache(t)

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "test-uuid", Alias: "0", CreatedAt: now},
		},
	})

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "uuid:test-uuid" && args[1] == "export" {
			io.WriteString(stdout, `[{"uuid":"test-uuid","description":"Task","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"modify", "test-uuid", "priority:H"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("modify code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "ok 0") || strings.Contains(stdout.String(), "test-uuid") {
		t.Fatalf("stdout = %q, want ok + alias only", stdout.String())
	}
}

func TestHandleModify_NumericID(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for numeric ID")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"modify", "123", "priority:H"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("modify code = %d, want 1 for numeric ID", code)
	}
}

func TestHandleAnnotate_Success(t *testing.T) {
	now := useIsolatedTaskAliasCache(t)

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "test-uuid", Alias: "0", CreatedAt: now},
		},
	})

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "uuid:test-uuid" && args[1] == "export" {
			io.WriteString(stdout, `[{"uuid":"test-uuid","description":"Task","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"annotate", "test-uuid", "new note"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("annotate code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "ok 0") || strings.Contains(stdout.String(), "test-uuid") {
		t.Fatalf("stdout = %q, want ok + alias only", stdout.String())
	}
}

func TestHandleAnnotate_MissingArgs(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for missing args")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"annotate", "uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("annotate code = %d, want 1 for missing args", code)
	}
	if !strings.Contains(stderr.String(), "requires an ID or UUID") {
		t.Fatalf("stderr = %q, want ID-or-UUID message", stderr.String())
	}
}

func TestHandleStart_Success(t *testing.T) {
	now := useIsolatedTaskAliasCache(t)

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "test-uuid", Alias: "0", CreatedAt: now},
		},
	})

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "uuid:test-uuid" && args[1] == "export" {
			io.WriteString(stdout, `[{"uuid":"test-uuid","description":"Task","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"start", "test-uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("start code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "ok 0") || strings.Contains(stdout.String(), "test-uuid") {
		t.Fatalf("stdout = %q, want ok + alias only", stdout.String())
	}
}

func TestHandleStart_MissingUUID(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for missing UUID")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"start"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("start code = %d, want 1 for missing UUID", code)
	}
	if !strings.Contains(stderr.String(), "requires an ID or UUID") {
		t.Fatalf("stderr = %q, want ID-or-UUID message", stderr.String())
	}
}

func TestHandleStop_Success(t *testing.T) {
	useIsolatedTaskAliasCache(t)

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "uuid:test-uuid" && args[1] == "export" {
			io.WriteString(stdout, `[{"uuid":"test-uuid","description":"Task","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"stop", "test-uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("stop code = %d, want 0", code)
	}
}

func TestHandleDone_Success(t *testing.T) {
	useIsolatedTaskAliasCache(t)

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "uuid:test-uuid" && args[1] == "export" {
			io.WriteString(stdout, `[{"uuid":"test-uuid","description":"Task","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"done", "test-uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("done code = %d, want 0", code)
	}
}

func TestHandlePriority_Success(t *testing.T) {
	useIsolatedTaskAliasCache(t)

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "uuid:test-uuid" && args[1] == "export" {
			io.WriteString(stdout, `[{"uuid":"test-uuid","description":"Task","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"priority", "test-uuid", "H"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("priority code = %d, want 0", code)
	}
}

func TestHandlePriority_MissingArgs(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for missing args")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"priority", "uuid"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("priority code = %d, want 1 for missing args", code)
	}
	if !strings.Contains(stderr.String(), "requires an ID or UUID") {
		t.Fatalf("stderr = %q, want ID-or-UUID message", stderr.String())
	}
}

func TestHandleTag_Success(t *testing.T) {
	useIsolatedTaskAliasCache(t)

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "uuid:test-uuid" && args[1] == "export" {
			io.WriteString(stdout, `[{"uuid":"test-uuid","description":"Task","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"tag", "test-uuid", "+cli"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("tag code = %d, want 0", code)
	}
}

func TestHandleTag_NumericID(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called for numeric ID")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"tag", "123", "+cli"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("tag code = %d, want 1 for numeric ID", code)
	}
}

func TestAllWriteHandlers_PassCorrectArgs(t *testing.T) {
	testCases := []struct {
		subcommand string
		args       []string
		wantArgs   []string
	}{
		// All commands use uuid:<uuid> as the filter so taskwarrior selects
		// the exact task; the action verb and any arguments follow.
		{"denotate", []string{"denotate", "my-uuid", "text"}, []string{"uuid:my-uuid", "denotate", "text"}},
		{"modify", []string{"modify", "my-uuid", "priority:H"}, []string{"uuid:my-uuid", "modify", "priority:H"}},
		{"annotate", []string{"annotate", "my-uuid", "note"}, []string{"uuid:my-uuid", "annotate", "note"}},
		{"start", []string{"start", "my-uuid"}, []string{"uuid:my-uuid", "start"}},
		{"stop", []string{"stop", "my-uuid"}, []string{"uuid:my-uuid", "stop"}},
		{"done", []string{"done", "my-uuid"}, []string{"uuid:my-uuid", "done"}},
		{"priority", []string{"priority", "my-uuid", "H"}, []string{"uuid:my-uuid", "modify", "priority:H"}},
		{"tag", []string{"tag", "my-uuid", "+cli"}, []string{"uuid:my-uuid", "modify", "+cli"}},
	}

	for _, tc := range testCases {
		t.Run(tc.subcommand, func(t *testing.T) {
			useIsolatedTaskAliasCache(t)

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
			d.Dispatch(context.Background(), tc.args, &bytes.Buffer{}, &stdout, &stderr)
			if len(capturedArgs) != len(tc.wantArgs) {
				t.Fatalf("capturedArgs = %v, want %v", capturedArgs, tc.wantArgs)
			}
			for i, want := range tc.wantArgs {
				if capturedArgs[i] != want {
					t.Fatalf("capturedArgs[%d] = %q, want %q", i, capturedArgs[i], want)
				}
			}
		})
	}
}

// TestAllWriteHandlers_AcceptUUIDPrefix verifies that commands accept a
// "uuid:" prefixed argument (e.g. "uuid:my-uuid") and strip the prefix
// before building the taskwarrior filter, so the filter is never doubled.
func TestAllWriteHandlers_AcceptUUIDPrefix(t *testing.T) {
	testCases := []struct {
		subcommand string
		args       []string
		wantArgs   []string
	}{
		{"denotate", []string{"denotate", "uuid:my-uuid", "text"}, []string{"uuid:my-uuid", "denotate", "text"}},
		{"modify", []string{"modify", "uuid:my-uuid", "priority:H"}, []string{"uuid:my-uuid", "modify", "priority:H"}},
		{"annotate", []string{"annotate", "uuid:my-uuid", "note"}, []string{"uuid:my-uuid", "annotate", "note"}},
		{"start", []string{"start", "uuid:my-uuid"}, []string{"uuid:my-uuid", "start"}},
		{"stop", []string{"stop", "uuid:my-uuid"}, []string{"uuid:my-uuid", "stop"}},
		{"done", []string{"done", "uuid:my-uuid"}, []string{"uuid:my-uuid", "done"}},
		{"priority", []string{"priority", "uuid:my-uuid", "H"}, []string{"uuid:my-uuid", "modify", "priority:H"}},
		{"tag", []string{"tag", "uuid:my-uuid", "+cli"}, []string{"uuid:my-uuid", "modify", "+cli"}},
	}

	for _, tc := range testCases {
		t.Run(tc.subcommand, func(t *testing.T) {
			useIsolatedTaskAliasCache(t)

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
			d.Dispatch(context.Background(), tc.args, &bytes.Buffer{}, &stdout, &stderr)
			if len(capturedArgs) != len(tc.wantArgs) {
				t.Fatalf("capturedArgs = %v, want %v", capturedArgs, tc.wantArgs)
			}
			for i, want := range tc.wantArgs {
				if capturedArgs[i] != want {
					t.Fatalf("capturedArgs[%d] = %q, want %q", i, capturedArgs[i], want)
				}
			}
		})
	}
}

func TestRunSingleTaskCommand_ResolveErrorWritesInfoError(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatal("runFn should not be called when selector resolution fails")
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.runSingleTaskCommand(
		context.Background(),
		"123",
		&stdout,
		&stderr,
		func(resolved resolvedTaskSelector) []string {
			t.Fatal("buildArgs should not be called when selector resolution fails")
			return nil
		},
	)
	if code != 1 {
		t.Fatalf("runSingleTaskCommand code = %d, want 1", code)
	}
	if err != nil {
		t.Fatalf("runSingleTaskCommand returned error: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty output", stdout.String())
	}
	if !strings.Contains(stderr.String(), "use a task alias ID or UUID") {
		t.Fatalf("stderr = %q, want numeric task ID error", stderr.String())
	}
}

func TestRunSingleTaskCommand_RunFailureDoesNotWriteSuccess(t *testing.T) {
	useIsolatedTaskAliasCache(t)

	runErr := errors.New("runner failed")
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "uuid:test-uuid" && args[1] == "export" {
			io.WriteString(stdout, `[{"uuid":"test-uuid","description":"Task","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		}
		return 2, runErr
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.runSingleTaskCommand(
		context.Background(),
		"test-uuid",
		&stdout,
		&stderr,
		func(resolved resolvedTaskSelector) []string {
			return []string{"uuid:" + resolved.UUID, "start"}
		},
	)
	if code != 2 {
		t.Fatalf("runSingleTaskCommand code = %d, want 2", code)
	}
	if !errors.Is(err, runErr) {
		t.Fatalf("runSingleTaskCommand error = %v, want %v", err, runErr)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty output", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty output", stderr.String())
	}
}
