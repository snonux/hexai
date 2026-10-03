package askcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestHandleProjects_ListsUniqueProjects(t *testing.T) {
	d := NewDispatcher(nil)
	d.findTaskBinary = func() (string, error) { return "task", nil }
	d.runTaskCommand = func(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		tasks := []TaskExport{
			{UUID: "1", Project: "hexai", Status: "pending", Urgency: 1},
			{UUID: "2", Project: "dtail", Status: "pending", Urgency: 2, Start: "2026-01-01T00:00:00Z"},
			{UUID: "3", Project: "hexai", Status: "pending", Urgency: 3},
			{UUID: "4", Project: "", Status: "pending", Urgency: 4},
		}
		_, _ = io.WriteString(stdout, taskExportJSON(tasks))
		return nil
	}

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(ctx, []string{"projects"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 1 || lines[0] != "hexai" {
		t.Fatalf("expected [hexai], got %q", lines)
	}
}

func TestHandleProjects_JSONOutput(t *testing.T) {
	d := NewDispatcher(nil)
	d.findTaskBinary = func() (string, error) { return "task", nil }
	d.runTaskCommand = func(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		tasks := []TaskExport{
			{UUID: "1", Project: "hexai", Status: "pending", Urgency: 1},
			{UUID: "2", Project: "dtail", Status: "pending", Urgency: 2},
		}
		_, _ = io.WriteString(stdout, taskExportJSON(tasks))
		return nil
	}

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(ctx, []string{"--json", "projects"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}

	if !strings.Contains(stdout.String(), `"dtail"`) || !strings.Contains(stdout.String(), `"hexai"`) {
		t.Fatalf("unexpected JSON output: %s", stdout.String())
	}
}

func TestHandleProjects_EmptyResult(t *testing.T) {
	d := NewDispatcher(nil)
	d.findTaskBinary = func() (string, error) { return "task", nil }
	d.runTaskCommand = func(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		_, _ = io.WriteString(stdout, "[]")
		return nil
	}

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(ctx, []string{"projects"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	if stdout.String() != "" {
		t.Fatalf("expected no output, got %q", stdout.String())
	}
}

func TestHandleProjects_ForwardsTaskExportError(t *testing.T) {
	d := NewDispatcher(nil)
	d.findTaskBinary = func() (string, error) { return "task", nil }
	d.runTaskCommand = func(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		return fmt.Errorf("some error")
	}

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(ctx, []string{"projects"}, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestHandleProjects_TagFiltersBeforeExport(t *testing.T) {
	d := NewDispatcher(nil)
	d.findTaskBinary = func() (string, error) { return "task", nil }
	var gotArgs []string
	d.runTaskCommand = func(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		gotArgs = append([]string(nil), args...)
		_, _ = io.WriteString(stdout, "[]")
		return nil
	}

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(ctx, []string{"projects", "+auto", "+cli"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}

	want := []string{
		"rc.verbose=nothing",
		"rc.confirmation=off",
		"+agent",
		"status:pending",
		"+auto",
		"+cli",
		"export",
	}
	if len(gotArgs) != len(want) {
		t.Fatalf("args = %v, want %v", gotArgs, want)
	}
	for i := range want {
		if gotArgs[i] != want[i] {
			t.Fatalf("args = %v, want %v", gotArgs, want)
		}
	}
}

func TestHandleProjects_IgnoresNonTagArgs(t *testing.T) {
	d := NewDispatcher(nil)
	d.findTaskBinary = func() (string, error) { return "task", nil }
	var gotArgs []string
	d.runTaskCommand = func(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		gotArgs = append([]string(nil), args...)
		_, _ = io.WriteString(stdout, "[]")
		return nil
	}

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(ctx, []string{"projects", "limit:5", "+auto", "sort:urgency-"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}

	want := []string{
		"rc.verbose=nothing",
		"rc.confirmation=off",
		"+agent",
		"status:pending",
		"+auto",
		"export",
	}
	if len(gotArgs) != len(want) {
		t.Fatalf("args = %v, want %v", gotArgs, want)
	}
	for i := range want {
		if gotArgs[i] != want[i] {
			t.Fatalf("args = %v, want %v", gotArgs, want)
		}
	}
}

func taskExportJSON(tasks []TaskExport) string {
	data, err := json.Marshal(tasks)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func TestHandleProjects_PassesDueWindowFilter(t *testing.T) {
	var captured []string
	d := NewDispatcher(nil)
	d.now = func() time.Time { return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) }
	d.findTaskBinary = func() (string, error) { return "task", nil }
	d.runTaskCommand = func(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		captured = args
		_, _ = io.WriteString(stdout, "[]")
		return nil
	}
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"projects", "+auto", "due-window:7.days"}, nil, &stdout, &stderr)
	if err != nil || code != 0 {
		t.Fatalf("code=%d err=%v stderr=%q", code, err, stderr.String())
	}
	want := "(due.none: or due.by:2026-09-05T12:00)"
	idx := -1
	for i, a := range captured {
		if a == want {
			idx = i
		}
	}
	if idx < 0 || captured[len(captured)-1] != "export" || idx > len(captured)-2 {
		t.Fatalf("expected %q before export, got %v", want, captured)
	}
}

func TestHandleProjects_InvalidDueFilter(t *testing.T) {
	called := false
	d := NewDispatcher(nil)
	// The value must be rejected before looking up taskwarrior at all.
	d.findTaskBinary = func() (string, error) { return "", fmt.Errorf("task not installed") }
	d.runTaskCommand = func(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		called = true
		return nil
	}
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"projects", "due-within:soon"}, nil, &stdout, &stderr)
	if code != 1 || called {
		t.Fatalf("expected exit 1 without running task, got code=%d called=%v", code, called)
	}
	if !strings.Contains(stderr.String(), "invalid due value") {
		t.Fatalf("expected invalid-due error, got %q", stderr.String())
	}
}
