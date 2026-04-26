package taskproxy

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

func TestRunnerRun_InjectsProjectFilterAndAgentTag(t *testing.T) {
	var gotName string
	var gotArgs []string
	runner := Runner{
		CommandName:    "ask",
		findTaskBinary: func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return "/tmp/work/hexai", nil },
		runCommand: func(_ context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			gotName = name
			gotArgs = append([]string(nil), args...)
			return nil
		},
	}

	exitCode, err := runner.Run(context.Background(), []string{"list", "limit:1"}, strings.NewReader("in"), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
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

func TestRunnerRun_OutsideGitRepo_IsActionable(t *testing.T) {
	runner := Runner{
		CommandName:    "ask",
		findTaskBinary: func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return "", errors.New("git failed") },
		runCommand: func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
			t.Fatal("runCommand should not be called when repo detection fails")
			return nil
		},
	}

	exitCode, err := runner.Run(context.Background(), []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if err == nil || !strings.Contains(err.Error(), "must be run inside a git repository") {
		t.Fatalf("expected actionable git-repo error, got %v", err)
	}
}

func TestRunnerRun_PreservesTaskwarriorExitCode(t *testing.T) {
	runner := Runner{
		CommandName:    "ask",
		findTaskBinary: func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return "/tmp/work/hexai", nil },
		runCommand: func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
			return exec.Command("sh", "-c", "exit 7").Run()
		},
	}

	exitCode, err := runner.Run(context.Background(), []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("expected nil error for subprocess exit, got %v", err)
	}
	if exitCode != 7 {
		t.Fatalf("exitCode = %d, want 7", exitCode)
	}
}

func TestRunnerRun_PreservesStdoutAndStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{
		CommandName:    "ask",
		findTaskBinary: func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return "/tmp/work/hexai", nil },
		runCommand: func(_ context.Context, name string, args []string, stdin io.Reader, out, errOut io.Writer) error {
			_, _ = io.WriteString(out, "task stdout")
			_, _ = io.WriteString(errOut, "task stderr")
			return nil
		},
	}

	exitCode, err := runner.Run(context.Background(), []string{"list"}, strings.NewReader(""), &stdout, &stderr)
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

func TestRunnerRun_TaskLookupFailure_IsActionable(t *testing.T) {
	runner := Runner{
		CommandName:    "ask",
		findTaskBinary: func() (string, error) { return "", errors.New("not found") },
	}

	exitCode, err := runner.Run(context.Background(), []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if err == nil || !strings.Contains(err.Error(), "Taskwarrior binary lookup failed") {
		t.Fatalf("expected actionable task lookup error, got %v", err)
	}
}

func TestRunnerRun_EmptyRepoName_IsActionable(t *testing.T) {
	runner := Runner{
		CommandName:    "ask",
		findTaskBinary: func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return "/", nil },
	}

	exitCode, err := runner.Run(context.Background(), []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if err == nil || !strings.Contains(err.Error(), "could not derive project name") {
		t.Fatalf("expected actionable project-name error, got %v", err)
	}
}

// NewRunner must wire all three function-typed fields and trim the command
// name; the rest of Runner relies on these defaults when callers don't
// override them in tests.
func TestNewRunner_DefaultsAreWiredAndCommandTrimmed(t *testing.T) {
	r := NewRunner("  ask  ")
	if r.CommandName != "ask" {
		t.Fatalf("CommandName = %q, want %q", r.CommandName, "ask")
	}
	if r.findTaskBinary == nil || r.detectRepoRoot == nil || r.runCommand == nil {
		t.Fatalf("NewRunner did not wire all defaults: %+v", r)
	}
}

// normalizeRunner must fall back to "task" when CommandName is empty; the
// branch is otherwise unreachable through the public Run path because callers
// always pass a label.
func TestNormalizeRunner_FillsDefaultsForZeroValue(t *testing.T) {
	got := normalizeRunner(Runner{})
	if got.CommandName != "task" {
		t.Fatalf("CommandName = %q, want %q", got.CommandName, "task")
	}
	if got.findTaskBinary == nil || got.detectRepoRoot == nil || got.runCommand == nil {
		t.Fatalf("normalizeRunner left a nil func field: %+v", got)
	}
	if label := got.commandLabel(); label != "task" {
		t.Fatalf("commandLabel = %q, want %q", label, "task")
	}
}

// Whitespace-only CommandName must be treated as unset by commandLabel; this
// keeps error messages readable when callers accidentally pass "   ".
func TestCommandLabel_TrimsToFallback(t *testing.T) {
	r := Runner{CommandName: "   "}
	if got := r.commandLabel(); got != "task" {
		t.Fatalf("commandLabel = %q, want %q", got, "task")
	}
}

// exitCodeFor must wrap non-ExitError failures (e.g. fork/exec errors) so the
// caller can distinguish "Taskwarrior ran and exited N" from "we never got to
// run Taskwarrior at all".
func TestExitCodeFor_NonExitError(t *testing.T) {
	r := Runner{CommandName: "ask"}
	code, err := r.exitCodeFor(errors.New("fork failed"))
	if code != 1 {
		t.Fatalf("exitCodeFor(non-ExitError) code = %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "failed to run Taskwarrior") {
		t.Fatalf("expected wrap message, got %v", err)
	}
}

// findTaskBinary success path: stage a fake "task" executable on PATH and
// confirm LookPath resolves to it.
func TestFindTaskBinary_FoundInPath(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "task")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	got, err := findTaskBinary()
	if err != nil {
		t.Fatalf("findTaskBinary: %v", err)
	}
	if got != fake {
		t.Fatalf("findTaskBinary = %q, want %q", got, fake)
	}
}

// findTaskBinary failure path: an empty PATH must yield the actionable
// "install Taskwarrior" error users see when the binary is missing.
func TestFindTaskBinary_NotFound_IsActionable(t *testing.T) {
	t.Setenv("PATH", "")
	_, err := findTaskBinary()
	if err == nil {
		t.Fatalf("expected error for missing task binary")
	}
	if !strings.Contains(err.Error(), "install Taskwarrior and retry") {
		t.Fatalf("error not actionable: %v", err)
	}
}

// detectRepoRoot must return the repo top level when invoked inside a real
// git checkout. Reuses the current process' git repository (this test file
// lives inside the hexai checkout) to avoid needing to git-init a temp dir.
func TestDetectRepoRoot_InsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root, err := detectRepoRoot(context.Background())
	if err != nil {
		t.Fatalf("detectRepoRoot: %v", err)
	}
	if root == "" || !strings.Contains(root, "hexai") {
		t.Fatalf("unexpected repo root %q", root)
	}
}

// detectRepoRoot outside a git repo: chdir into a temp dir that has no .git
// and confirm the actionable error fires. We restore cwd via t.Chdir which
// the test runner reverts automatically on test exit.
func TestDetectRepoRoot_OutsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	_, err := detectRepoRoot(context.Background())
	if err == nil {
		t.Fatalf("expected error outside git repo")
	}
	if !strings.Contains(err.Error(), "must be run inside a git repository") {
		t.Fatalf("error not actionable: %v", err)
	}
}

// runTaskCommand must wire stdin/stdout/stderr to the spawned process. We
// invoke /bin/sh to echo from stdin and confirm both streams round-trip.
func TestRunTaskCommand_StreamsAndReturnNil(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	var stdout, stderr bytes.Buffer
	err := runTaskCommand(
		context.Background(),
		"sh",
		[]string{"-c", "cat; echo err 1>&2"},
		strings.NewReader("hello\n"),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("runTaskCommand returned error: %v", err)
	}
	if got := stdout.String(); got != "hello\n" {
		t.Fatalf("stdout = %q, want %q", got, "hello\n")
	}
	if got := stderr.String(); got != "err\n" {
		t.Fatalf("stderr = %q, want %q", got, "err\n")
	}
}

// runTaskCommand surfaces the underlying exec error when the binary doesn't
// exist; Run() relies on this to map to a non-zero exit code.
func TestRunTaskCommand_BinaryNotFound(t *testing.T) {
	err := runTaskCommand(context.Background(), "/no/such/binary", nil, nil, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("expected error from missing binary")
	}
}
