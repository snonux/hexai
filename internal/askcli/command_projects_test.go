package askcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
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
