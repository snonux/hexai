package askcli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func stubEditorCapture(t *testing.T, content string, err error) {
	t.Helper()
	old := captureFromEditor
	captureFromEditor = func(_ context.Context, initial []byte) (string, error) {
		return content, err
	}
	t.Cleanup(func() { captureFromEditor = old })
}

func TestHandleEdit_Success(t *testing.T) {
	now := useIsolatedTaskAliasCache(t)
	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "existing-uuid", Alias: "0", CreatedAt: now},
		},
	})
	// editor.OpenTempAndEdit trims content, so mimic that here.
	stubEditorCapture(t, "Multi line\ntask description", nil)

	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		capturedArgs = args
		_, _ = io.WriteString(stdout, "Created task abc-123-def.")
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"edit"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("edit code = %d stderr = %q", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "created task 1" {
		t.Fatalf("stdout = %q, want created task 1", stdout.String())
	}
	if len(capturedArgs) == 0 || capturedArgs[len(capturedArgs)-1] != "Multi line\ntask description" {
		t.Fatalf("description arg = %v, want trimmed multi-line content", capturedArgs)
	}
}

func TestHandleEdit_ExistingTaskModifiesDescription(t *testing.T) {
	now := useIsolatedTaskAliasCache(t)
	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "existing-uuid", Alias: "0", CreatedAt: now},
		},
	})

	var initialContent []byte
	old := captureFromEditor
	captureFromEditor = func(_ context.Context, initial []byte) (string, error) {
		initialContent = initial
		return "updated description", nil
	}
	t.Cleanup(func() { captureFromEditor = old })

	var capturedArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		// First call: export to resolve selector. Subsequent: modify.
		if len(args) >= 2 && args[1] == "export" {
			_, _ = io.WriteString(stdout, `[{"uuid":"existing-uuid","description":"old description","status":"pending"}]`)
			return 0, nil
		}
		capturedArgs = args
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"edit", "0"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("edit code = %d stderr = %q", code, stderr.String())
	}
	if string(initialContent) != "old description" {
		t.Fatalf("editor initial content = %q, want old description", initialContent)
	}
	want := []string{"uuid:existing-uuid", "modify", "updated description"}
	if len(capturedArgs) != len(want) {
		t.Fatalf("modify args = %v, want %v", capturedArgs, want)
	}
	for i := range want {
		if capturedArgs[i] != want[i] {
			t.Fatalf("modify args = %v, want %v", capturedArgs, want)
		}
	}
}

func TestHandleEdit_EmptyContentAborts(t *testing.T) {
	stubEditorCapture(t, "", nil)

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatalf("runner should not be called on empty content")
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"edit"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("edit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "empty description") {
		t.Fatalf("stderr = %q, want empty description error", stderr.String())
	}
}

func TestHandleEdit_EditorError(t *testing.T) {
	stubEditorCapture(t, "", errors.New("boom"))

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatalf("runner should not be called when editor fails")
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"edit"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("edit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("stderr = %q, want editor error", stderr.String())
	}
}
