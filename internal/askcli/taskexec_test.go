package askcli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fakeHexaiRepoDir(t *testing.T) string {
	t.Helper()
	base := filepath.Join(t.TempDir(), "hexai")
	if err := os.MkdirAll(filepath.Join(base, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	return base
}

func TestExecutorTaskArgs(t *testing.T) {
	exec_ := NewExecutor("ask")
	args, err := exec_.taskArgs(context.Background(), "/tmp/work/hexai", []string{"list", "limit:1"})
	if err != nil {
		t.Fatalf("taskArgs returned error: %v", err)
	}
	want := []string{"rc.verbose=nothing", "rc.confirmation=off", "project:hexai", "+agent", "list", "limit:1"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("task args = %v, want %v", args, want)
	}
}

func TestExecutorTaskArgs_NoAgentScope(t *testing.T) {
	exec_ := NewExecutor("ask")
	ctx := contextWithTaskScope(context.Background(), taskScopeNoAgent)
	args, err := exec_.taskArgs(ctx, "/tmp/work/hexai", []string{"list", "limit:1"})
	if err != nil {
		t.Fatalf("taskArgs returned error: %v", err)
	}
	want := []string{"rc.verbose=nothing", "rc.confirmation=off", "project:hexai", "-agent", "list", "limit:1"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("task args = %v, want %v", args, want)
	}
}

func TestExecutorTaskArgs_ProjectOverride(t *testing.T) {
	exec_ := NewExecutor("ask")
	ctx := contextWithTaskProject(context.Background(), "alpha")
	args, err := exec_.taskArgs(ctx, "", []string{"list", "limit:1"})
	if err != nil {
		t.Fatalf("taskArgs returned error: %v", err)
	}
	want := []string{"rc.verbose=nothing", "rc.confirmation=off", "project:alpha", "+agent", "list", "limit:1"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("task args = %v, want %v", args, want)
	}
}

func TestExecutorTaskArgs_AddDefaultScope(t *testing.T) {
	exec_ := NewExecutor("ask")
	args, err := exec_.taskArgs(context.Background(), "/tmp/work/hexai", []string{"add", "rc.verbose=nothing", "rc.verbose=new-uuid", "new task"})
	if err != nil {
		t.Fatalf("taskArgs returned error: %v", err)
	}
	want := []string{"rc.verbose=nothing", "rc.confirmation=off", "project:hexai", "add", "rc.verbose=nothing", "rc.verbose=new-uuid", "+agent", "new task"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("task args = %v, want %v", args, want)
	}
}

func TestExecutorTaskArgs_AddNoAgentScope(t *testing.T) {
	exec_ := NewExecutor("ask")
	ctx := contextWithTaskScope(context.Background(), taskScopeNoAgent)
	args, err := exec_.taskArgs(ctx, "/tmp/work/hexai", []string{"add", "rc.verbose=nothing", "rc.verbose=new-uuid", "new task"})
	if err != nil {
		t.Fatalf("taskArgs returned error: %v", err)
	}
	want := []string{"rc.verbose=nothing", "rc.confirmation=off", "project:hexai", "add", "rc.verbose=nothing", "rc.verbose=new-uuid", "new task"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("task args = %v, want %v", args, want)
	}
}

func TestExecutorRun_InjectsProjectFilterAndAgentTag(t *testing.T) {
	repo := fakeHexaiRepoDir(t)
	var gotName string
	var gotArgs []string
	exec_ := Executor{
		commandName:    "ask",
		findBinary:     func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return repo, nil },
		runCommand: func(_ context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			gotName = name
			gotArgs = append([]string(nil), args...)
			return nil
		},
	}

	exitCode, err := exec_.Run(context.Background(), []string{"list", "limit:1"}, strings.NewReader("in"), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if gotName != "/usr/bin/task" {
		t.Fatalf("task binary = %q, want /usr/bin/task", gotName)
	}
	wantArgs := []string{"rc.verbose=nothing", "rc.confirmation=off", "project:hexai", "+agent", "list", "limit:1"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("task args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestExecutorRun_InjectsProjectFilterAndNoAgentTag(t *testing.T) {
	repo := fakeHexaiRepoDir(t)
	var gotArgs []string
	exec_ := Executor{
		commandName:    "ask",
		findBinary:     func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return repo, nil },
		runCommand: func(_ context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			gotArgs = append([]string(nil), args...)
			return nil
		},
	}

	ctx := contextWithTaskScope(context.Background(), taskScopeNoAgent)
	exitCode, err := exec_.Run(ctx, []string{"list", "limit:1"}, strings.NewReader("in"), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	wantArgs := []string{"rc.verbose=nothing", "rc.confirmation=off", "project:hexai", "-agent", "list", "limit:1"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("task args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestExecutorRun_ProjectOverrideStillLocksUsingGitRoot(t *testing.T) {
	repo := fakeHexaiRepoDir(t)
	var detectCalls int
	var gotArgs []string
	exec_ := Executor{
		commandName: "ask",
		findBinary:  func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) {
			detectCalls++
			return repo, nil
		},
		runCommand: func(_ context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			gotArgs = append([]string(nil), args...)
			return nil
		},
	}

	ctx := contextWithTaskProject(context.Background(), "alpha")
	exitCode, err := exec_.Run(ctx, []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if detectCalls != 1 {
		t.Fatalf("detectRepoRoot calls = %d, want 1", detectCalls)
	}
	wantArgs := []string{"rc.verbose=nothing", "rc.confirmation=off", "project:alpha", "+agent", "list"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("task args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestExecutorRun_OutsideGitRepo_IsActionable(t *testing.T) {
	exec_ := Executor{
		commandName:    "ask",
		findBinary:     func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return "", errors.New("git failed") },
		runCommand: func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
			t.Fatal("runCommand should not be called when repo detection fails")
			return nil
		},
	}

	exitCode, err := exec_.Run(context.Background(), []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if err == nil || !strings.Contains(err.Error(), "must be run inside a git repository") {
		t.Fatalf("expected actionable git-repo error, got %v", err)
	}
}

func TestExecutorRun_PreservesTaskwarriorExitCode(t *testing.T) {
	repo := fakeHexaiRepoDir(t)
	exec_ := Executor{
		commandName:    "ask",
		findBinary:     func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return repo, nil },
		runCommand: func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
			return exec.Command("sh", "-c", "exit 7").Run()
		},
	}

	exitCode, err := exec_.Run(context.Background(), []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("expected nil error for subprocess exit, got %v", err)
	}
	if exitCode != 7 {
		t.Fatalf("exitCode = %d, want 7", exitCode)
	}
}

func TestExecutorRun_PreservesStdoutAndStderr(t *testing.T) {
	repo := fakeHexaiRepoDir(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exec_ := Executor{
		commandName:    "ask",
		findBinary:     func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return repo, nil },
		runCommand: func(_ context.Context, name string, args []string, stdin io.Reader, out, errOut io.Writer) error {
			_, _ = io.WriteString(out, "task stdout")
			_, _ = io.WriteString(errOut, "task stderr")
			return nil
		},
	}

	exitCode, err := exec_.Run(context.Background(), []string{"list"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
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

func TestExecutorRun_TaskLookupFailure_IsActionable(t *testing.T) {
	exec_ := Executor{
		commandName: "ask",
		findBinary:  func() (string, error) { return "", errors.New("not found") },
	}

	exitCode, err := exec_.Run(context.Background(), []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if err == nil || !strings.Contains(err.Error(), "task binary lookup failed") {
		t.Fatalf("expected actionable task lookup error, got %v", err)
	}
}

func TestExecutorRun_EmptyRepoName_IsActionable(t *testing.T) {
	exec_ := Executor{
		commandName:    "ask",
		findBinary:     func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return "/", nil },
	}

	exitCode, err := exec_.Run(context.Background(), []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if err == nil || !strings.Contains(err.Error(), "could not derive project name") {
		t.Fatalf("expected actionable project-name error, got %v", err)
	}
}
