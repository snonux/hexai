package askcli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestCountScopedPendingTasks(t *testing.T) {
	for _, prefix := range [][]string{nil, {"na"}, {"proj:other"}} {
		t.Run(strings.Join(prefix, " "), func(t *testing.T) {
			executor := NewExecutor("ask")
			executor.findBinary = func() (string, error) { return "task", nil }
			executor.detectRepoRoot = func(context.Context) (string, error) { return "", errors.New("no repo") }
			executor.getWorkingDir = func() (string, error) { return "/repo", nil }
			// Supply the project in context to avoid filesystem locks in this unit test.
			executor.runCommand = func(_ context.Context, _ string, args []string, _ io.Reader, stdout, _ io.Writer) error {
				project, scope := "repo", "+agent"
				if len(prefix) > 0 && prefix[0] == "na" {
					scope = "-agent"
				}
				if len(prefix) > 0 && prefix[0] == "proj:other" {
					project = "other"
				}
				want := []string{"rc.verbose=nothing", "rc.confirmation=off", projectReadFilter(project), scope, "status:pending", "count"}
				if !slices.Equal(args, want) {
					t.Fatalf("args = %q, want %q", args, want)
				}
				_, err := io.WriteString(stdout, "7\n")
				return err
			}
			var stdout, stderr bytes.Buffer
			ctx := contextWithTaskProject(context.Background(), "repo")
			code, err := NewDispatcher(executor).Dispatch(ctx, append(slices.Clone(prefix), "count"), nil, &stdout, &stderr)
			if code != 0 || err != nil || stdout.String() != "7\n" || stderr.Len() != 0 {
				t.Fatalf("count = %d, %v, stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCountRejectsArguments(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(context.Context, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		t.Fatal("unexpected runner call")
		return 0, nil
	}})
	var stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"count", "status:completed"}, nil, io.Discard, &stderr)
	if code != 1 || err != nil || !strings.Contains(stderr.String(), "usage: ask count") {
		t.Fatalf("result = %d, %v, %q", code, err, stderr.String())
	}
}
