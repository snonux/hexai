package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestRunTaskSubcommandIfRequested_SkipsNonTaskArgs(t *testing.T) {
	handled, exitCode, err := runTaskSubcommandIfRequested([]string{"hello"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if handled || exitCode != 0 || err != nil {
		t.Fatalf("expected non-task args to be ignored, got handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
}

func TestTaskRunnerRun_InjectsProjectFilterAndAgentTag(t *testing.T) {
	var gotName string
	var gotArgs []string
	runner := taskRunner{
		findTaskBinary: func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return "/tmp/work/hexai", nil },
		runCommand: func(_ context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			gotName = name
			gotArgs = append([]string(nil), args...)
			return nil
		},
	}

	exitCode, err := runner.run(context.Background(), []string{"list", "limit:1"}, strings.NewReader("in"), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if gotName != "/usr/bin/task" {
		t.Fatalf("task binary = %q, want /usr/bin/task", gotName)
	}
	wantArgs := []string{"project:hexai", "+agent", "list", "limit:1"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("task args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestTaskRunnerRun_OutsideGitRepo_IsActionable(t *testing.T) {
	runner := taskRunner{
		findTaskBinary: func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return "", errors.New("git failed") },
		runCommand: func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
			t.Fatal("runCommand should not be called when repo detection fails")
			return nil
		},
	}

	exitCode, err := runner.run(context.Background(), []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if err == nil || !strings.Contains(err.Error(), "must be run inside a git repository") {
		t.Fatalf("expected actionable git-repo error, got %v", err)
	}
}

func TestTaskRunnerRun_PreservesTaskwarriorExitCode(t *testing.T) {
	runner := taskRunner{
		findTaskBinary: func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return "/tmp/work/hexai", nil },
		runCommand: func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
			return exec.Command("sh", "-c", "exit 7").Run()
		},
	}

	exitCode, err := runner.run(context.Background(), []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("expected nil error for subprocess exit, got %v", err)
	}
	if exitCode != 7 {
		t.Fatalf("exitCode = %d, want 7", exitCode)
	}
}

func TestTaskRunnerRun_PreservesStdoutAndStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := taskRunner{
		findTaskBinary: func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return "/tmp/work/hexai", nil },
		runCommand: func(_ context.Context, name string, args []string, stdin io.Reader, out, errOut io.Writer) error {
			_, _ = io.WriteString(out, "task stdout")
			_, _ = io.WriteString(errOut, "task stderr")
			return nil
		},
	}

	exitCode, err := runner.run(context.Background(), []string{"list"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if stdout.String() != "task stdout" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "task stdout")
	}
	if stderr.String() != "task stderr" {
		t.Fatalf("stderr = %q, want %q", stderr.String(), "task stderr")
	}
}

func TestTaskRunnerRun_TaskLookupFailure_IsActionable(t *testing.T) {
	runner := taskRunner{
		findTaskBinary: func() (string, error) { return "", errors.New("not found") },
	}

	exitCode, err := runner.run(context.Background(), []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if err == nil || !strings.Contains(err.Error(), "Taskwarrior binary lookup failed") {
		t.Fatalf("expected actionable task lookup error, got %v", err)
	}
}

func TestTaskRunnerRun_EmptyRepoName_IsActionable(t *testing.T) {
	runner := taskRunner{
		findTaskBinary: func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return "/", nil },
	}

	exitCode, err := runner.run(context.Background(), []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if err == nil || !strings.Contains(err.Error(), "could not derive project name") {
		t.Fatalf("expected actionable project-name error, got %v", err)
	}
}
