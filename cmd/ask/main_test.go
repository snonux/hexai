package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestAskRunnerRun_ForwardsArgs(t *testing.T) {
	var gotArgs []string
	var stdout bytes.Buffer
	runner := askRunner{
		runTask: func(_ context.Context, args []string, stdin io.Reader, out, errOut io.Writer) (int, error) {
			gotArgs = append([]string(nil), args...)
			_, _ = io.WriteString(out, "task output")
			return 0, nil
		},
	}

	exitCode := runner.run([]string{"list", "limit:1"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	wantArgs := []string{"list", "limit:1"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args = %v, want %v", gotArgs, wantArgs)
	}
	if stdout.String() != "task output" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "task output")
	}
}

func TestAskRunnerRun_WritesErrorToStderr(t *testing.T) {
	var stderr bytes.Buffer
	runner := askRunner{
		runTask: func(context.Context, []string, io.Reader, io.Writer, io.Writer) (int, error) {
			return 1, errors.New("ask: must be run inside a git repository")
		},
	}

	exitCode := runner.run([]string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "must be run inside a git repository") {
		t.Fatalf("stderr = %q, want actionable repo guidance", stderr.String())
	}
}
