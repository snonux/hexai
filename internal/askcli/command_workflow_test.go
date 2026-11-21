package askcli

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
)

// workflowDispatcher resolves "test-uuid" and records every non-export
// Taskwarrior call. failStop makes the "stop" call fail, as Taskwarrior does
// for a task that was never started.
func workflowDispatcher(t *testing.T, failStop bool) (*Dispatcher, *[][]string) {
	t.Helper()
	useIsolatedTaskAliasCache(t)
	var calls [][]string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "uuid:test-uuid" && args[1] == "export" {
			_, _ = io.WriteString(stdout, `[{"uuid":"test-uuid","description":"Task","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
			return 0, nil
		}
		calls = append(calls, append([]string(nil), args...))
		if failStop && len(args) == 2 && args[1] == "stop" {
			return 1, nil
		}
		return 0, nil
	}})
	return d, &calls
}

func runWorkflow(t *testing.T, d *Dispatcher, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), args, &bytes.Buffer{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Dispatch(%v) returned error: %v", args, err)
	}
	return code, stdout.String(), stderr.String()
}

func TestHandleStop_WithNoteAnnotatesFirst(t *testing.T) {
	d, calls := workflowDispatcher(t, false)
	if code, _, stderr := runWorkflow(t, d, "stop", "test-uuid", "Handoff:", "tests red in parser"); code != 0 {
		t.Fatalf("stop with note code = %d, stderr = %s", code, stderr)
	}
	want := [][]string{
		{"uuid:test-uuid", "annotate", "Handoff: tests red in parser"},
		{"uuid:test-uuid", "stop"},
	}
	if !reflect.DeepEqual(*calls, want) {
		t.Fatalf("calls = %v, want %v", *calls, want)
	}
}

func TestHandleDone_WithNoteAnnotatesFirst(t *testing.T) {
	d, calls := workflowDispatcher(t, false)
	if code, _, stderr := runWorkflow(t, d, "done", "test-uuid", "Delivered: parser fix, tests green"); code != 0 {
		t.Fatalf("done with note code = %d, stderr = %s", code, stderr)
	}
	want := [][]string{
		{"uuid:test-uuid", "annotate", "Delivered: parser fix, tests green"},
		{"uuid:test-uuid", "done"},
	}
	if !reflect.DeepEqual(*calls, want) {
		t.Fatalf("calls = %v, want %v", *calls, want)
	}
}

func TestHandleDone_WithoutNoteOnlyCompletes(t *testing.T) {
	d, calls := workflowDispatcher(t, false)
	if code, _, _ := runWorkflow(t, d, "done", "test-uuid"); code != 0 {
		t.Fatalf("done code = %d", code)
	}
	if want := [][]string{{"uuid:test-uuid", "done"}}; !reflect.DeepEqual(*calls, want) {
		t.Fatalf("calls = %v, want %v", *calls, want)
	}
}

// Parking must work for a task that was never started: the stop step fails
// in Taskwarrior then, and that failure must not abort the park.
func TestHandlePark_StopsTagsAndRecordsQuestion(t *testing.T) {
	for _, failStop := range []bool{false, true} {
		d, calls := workflowDispatcher(t, failStop)
		code, stdout, stderr := runWorkflow(t, d, "park", "test-uuid", "which", "port?")
		if code != 0 {
			t.Fatalf("park (failStop=%v) code = %d, stderr = %s", failStop, code, stderr)
		}
		want := [][]string{
			{"uuid:test-uuid", "stop"},
			{"uuid:test-uuid", "modify", "+" + needsHumanTag},
			{"uuid:test-uuid", "annotate", "Needs human: which port?"},
		}
		if !reflect.DeepEqual(*calls, want) {
			t.Fatalf("park (failStop=%v) calls = %v, want %v", failStop, *calls, want)
		}
		if !strings.HasPrefix(stdout, "ok ") {
			t.Fatalf("park stdout = %q, want ok <id>", stdout)
		}
	}
}

func TestHandlePark_KeepsExistingPrefix(t *testing.T) {
	d, calls := workflowDispatcher(t, false)
	runWorkflow(t, d, "park", "test-uuid", "Needs human: approve the reboot")
	last := (*calls)[len(*calls)-1]
	if got := last[2]; got != "Needs human: approve the reboot" {
		t.Fatalf("annotation = %q, prefix must not be doubled", got)
	}
}

func TestHandlePark_RequiresQuestion(t *testing.T) {
	d, calls := workflowDispatcher(t, false)
	code, _, stderr := runWorkflow(t, d, "park", "test-uuid")
	if code == 0 || !strings.Contains(stderr, "ask park") {
		t.Fatalf("park without question: code = %d, stderr = %q", code, stderr)
	}
	if len(*calls) != 0 {
		t.Fatalf("park without question must not touch the task: %v", *calls)
	}
}

func TestHandleUnpark_RemovesTag(t *testing.T) {
	d, calls := workflowDispatcher(t, false)
	if code, _, stderr := runWorkflow(t, d, "unpark", "test-uuid"); code != 0 {
		t.Fatalf("unpark code = %d, stderr = %s", code, stderr)
	}
	if want := [][]string{{"uuid:test-uuid", "modify", "-" + needsHumanTag}}; !reflect.DeepEqual(*calls, want) {
		t.Fatalf("calls = %v, want %v", *calls, want)
	}
}

// listFilterArgs runs a list-style subcommand and returns the Taskwarrior
// filter it issued.
func listFilterArgs(t *testing.T, args ...string) []string {
	t.Helper()
	useIsolatedTaskAliasCache(t)
	var got []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, a []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		got = append([]string(nil), a...)
		_, _ = io.WriteString(stdout, "[]")
		return 0, nil
	}})
	if code, _, stderr := runWorkflow(t, d, args...); code != 0 {
		t.Fatalf("%v code = %d, stderr = %s", args, code, stderr)
	}
	return got
}

func TestHandleReady_HidesParkedTasks(t *testing.T) {
	got := listFilterArgs(t, "ready")
	if want := []string{"+READY", "-" + needsHumanTag, "export"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ready filter = %v, want %v", got, want)
	}
}

func TestHandleParked_ListsOnlyParkedPendingTasks(t *testing.T) {
	got := listFilterArgs(t, "parked")
	if want := []string{"status:pending", "+" + needsHumanTag, "export"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parked filter = %v, want %v", got, want)
	}
}

// A single unknown word is a mistyped subcommand ("ask tasks", "ask ls"), not
// a task description, so it must not create a task in agent scope.
func TestDispatcher_RefusesSingleWordImplicitAdd(t *testing.T) {
	for _, word := range []string{"tasks", "ls", "status"} {
		useIsolatedTaskAliasCache(t)
		var calls [][]string
		d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, a []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			calls = append(calls, append([]string(nil), a...))
			return 0, nil
		}})
		code, _, stderr := runWorkflow(t, d, word)
		if code == 0 {
			t.Fatalf("ask %s must fail", word)
		}
		for _, call := range calls {
			if call[0] == "add" {
				t.Fatalf("ask %s created a task: %v", word, call)
			}
		}
		if !strings.Contains(stderr, "ask add") || !strings.Contains(stderr, "ask help") {
			t.Fatalf("ask %s stderr = %q, want a hint naming ask add and ask help", word, stderr)
		}
	}
}

func TestDispatcher_SingleWordImplicitAddStillWorksWithoutAgentScope(t *testing.T) {
	useIsolatedTaskAliasCache(t)
	added := false
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, a []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(a) > 0 && a[0] == "add" {
			added = true
			_, _ = io.WriteString(stdout, "Created task task-uuid-abc.\n")
		}
		return 0, nil
	}})
	if code, _, stderr := runWorkflow(t, d, "na", "milk"); code != 0 {
		t.Fatalf("ask na milk code = %d, stderr = %s", code, stderr)
	}
	if !added {
		t.Fatal("ask na milk must still create a personal task")
	}
}
