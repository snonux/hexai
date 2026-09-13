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

func fixedWorkingDir(dir string) workingDirectory {
	return func() (string, error) { return dir, nil }
}

func TestProjectNameFromRoot(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		root    string
		cwd     string
		want    string
		wantErr string
	}{
		{name: "at root", root: "/tmp/work/dotfiles", cwd: "/tmp/work/dotfiles", want: "dotfiles"},
		{name: "one level", root: "/tmp/work/dotfiles", cwd: "/tmp/work/dotfiles/prompts", want: "dotfiles.prompts"},
		{name: "nested", root: "/tmp/work/dotfiles", cwd: "/tmp/work/dotfiles/a/b", want: "dotfiles.a.b"},
		{name: "dotted dirname", root: "/tmp/work/dotfiles", cwd: "/tmp/work/dotfiles/foo.bar/baz", want: "dotfiles.foo.bar.baz"},
		{name: "outside root", root: "/tmp/work/dotfiles", cwd: "/tmp/other", wantErr: "outside git root"},
		{name: "empty root base", root: "/", cwd: "/", wantErr: "could not derive project name"},
		{name: "empty cwd", root: "/tmp/work/dotfiles", cwd: "", wantErr: "working directory is empty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := projectNameFromRoot(tc.root, tc.cwd)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("projectNameFromRoot: %v", err)
			}
			if got != tc.want {
				t.Fatalf("project = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProjectReadFilter(t *testing.T) {
	t.Parallel()
	got := projectReadFilter("dotfiles.prompts")
	want := "(project.is:dotfiles.prompts or project:dotfiles.prompts.)"
	if got != want {
		t.Fatalf("filter = %q, want %q", got, want)
	}
}

func TestExecutorTaskArgs(t *testing.T) {
	repo := "/tmp/work/hexai"
	exec_ := Executor{
		commandName:   "ask",
		getWorkingDir: fixedWorkingDir(repo),
	}
	args, err := exec_.taskArgs(context.Background(), repo, []string{"list", "limit:1"})
	if err != nil {
		t.Fatalf("taskArgs returned error: %v", err)
	}
	want := []string{"rc.verbose=nothing", "rc.confirmation=off", "(project.is:hexai or project:hexai.)", "+agent", "list", "limit:1"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("task args = %v, want %v", args, want)
	}
}

func TestExecutorTaskArgs_SubdirectoryProject(t *testing.T) {
	repo := "/tmp/work/dotfiles"
	cwd := "/tmp/work/dotfiles/prompts"
	exec_ := Executor{
		commandName:   "ask",
		getWorkingDir: fixedWorkingDir(cwd),
	}
	args, err := exec_.taskArgs(context.Background(), repo, []string{"list"})
	if err != nil {
		t.Fatalf("taskArgs returned error: %v", err)
	}
	want := []string{"rc.verbose=nothing", "rc.confirmation=off", "(project.is:dotfiles.prompts or project:dotfiles.prompts.)", "+agent", "list"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("task args = %v, want %v", args, want)
	}
}

func TestExecutorTaskArgs_NoAgentScope(t *testing.T) {
	repo := "/tmp/work/hexai"
	exec_ := Executor{
		commandName:   "ask",
		getWorkingDir: fixedWorkingDir(repo),
	}
	ctx := contextWithTaskScope(context.Background(), taskScopeNoAgent)
	args, err := exec_.taskArgs(ctx, repo, []string{"list", "limit:1"})
	if err != nil {
		t.Fatalf("taskArgs returned error: %v", err)
	}
	want := []string{"rc.verbose=nothing", "rc.confirmation=off", "(project.is:hexai or project:hexai.)", "-agent", "list", "limit:1"}
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
	want := []string{"rc.verbose=nothing", "rc.confirmation=off", "(project.is:alpha or project:alpha.)", "+agent", "list", "limit:1"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("task args = %v, want %v", args, want)
	}
}

func TestExecutorTaskArgs_ProjectOverrideAdd(t *testing.T) {
	exec_ := NewExecutor("ask")
	ctx := contextWithTaskProject(context.Background(), "dotfiles.prompts")
	args, err := exec_.taskArgs(ctx, "", []string{"add", "new task"})
	if err != nil {
		t.Fatalf("taskArgs returned error: %v", err)
	}
	want := []string{"rc.verbose=nothing", "rc.confirmation=off", "project:dotfiles.prompts", "add", "+agent", "new task"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("task args = %v, want %v", args, want)
	}
}

func TestExecutorTaskArgs_WorkingDirFailure(t *testing.T) {
	exec_ := Executor{
		commandName: "ask",
		getWorkingDir: func() (string, error) {
			return "", errors.New("cwd unavailable")
		},
	}
	_, err := exec_.taskArgs(context.Background(), "/tmp/work/hexai", []string{"list"})
	if err == nil || !strings.Contains(err.Error(), "cwd unavailable") {
		t.Fatalf("expected cwd failure, got %v", err)
	}
}

func TestProjectNameFromRoot_SymlinkCheckout(t *testing.T) {
	physical := fakeHexaiRepoDir(t)
	sub := filepath.Join(physical, "prompts")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	linkParent := t.TempDir()
	link := filepath.Join(linkParent, "hexai-link")
	if err := os.Symlink(physical, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	got, err := projectNameFromRoot(physical, filepath.Join(link, "prompts"))
	if err != nil {
		t.Fatalf("projectNameFromRoot: %v", err)
	}
	if got != "hexai.prompts" {
		t.Fatalf("project = %q, want hexai.prompts", got)
	}
}

func TestExecutorTaskArgs_AddDefaultScope(t *testing.T) {
	repo := "/tmp/work/hexai"
	exec_ := Executor{
		commandName:   "ask",
		getWorkingDir: fixedWorkingDir(repo),
	}
	args, err := exec_.taskArgs(context.Background(), repo, []string{"add", "rc.verbose=nothing", "rc.verbose=new-uuid", "new task"})
	if err != nil {
		t.Fatalf("taskArgs returned error: %v", err)
	}
	want := []string{"rc.verbose=nothing", "rc.confirmation=off", "project:hexai", "add", "rc.verbose=nothing", "rc.verbose=new-uuid", "+agent", "new task"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("task args = %v, want %v", args, want)
	}
}

func TestExecutorTaskArgs_AddSubdirectoryProject(t *testing.T) {
	repo := "/tmp/work/dotfiles"
	cwd := "/tmp/work/dotfiles/prompts/nested"
	exec_ := Executor{
		commandName:   "ask",
		getWorkingDir: fixedWorkingDir(cwd),
	}
	args, err := exec_.taskArgs(context.Background(), repo, []string{"add", "new task"})
	if err != nil {
		t.Fatalf("taskArgs returned error: %v", err)
	}
	want := []string{"rc.verbose=nothing", "rc.confirmation=off", "project:dotfiles.prompts.nested", "add", "+agent", "new task"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("task args = %v, want %v", args, want)
	}
}

func TestExecutorTaskArgs_AddNoAgentScope(t *testing.T) {
	repo := "/tmp/work/hexai"
	exec_ := Executor{
		commandName:   "ask",
		getWorkingDir: fixedWorkingDir(repo),
	}
	ctx := contextWithTaskScope(context.Background(), taskScopeNoAgent)
	args, err := exec_.taskArgs(ctx, repo, []string{"add", "rc.verbose=nothing", "rc.verbose=new-uuid", "new task"})
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
		getWorkingDir:  fixedWorkingDir(repo),
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
	wantArgs := []string{"rc.verbose=nothing", "rc.confirmation=off", "(project.is:hexai or project:hexai.)", "+agent", "list", "limit:1"}
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
		getWorkingDir:  fixedWorkingDir(repo),
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
	wantArgs := []string{"rc.verbose=nothing", "rc.confirmation=off", "(project.is:hexai or project:hexai.)", "-agent", "list", "limit:1"}
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
	wantArgs := []string{"rc.verbose=nothing", "rc.confirmation=off", "(project.is:alpha or project:alpha.)", "+agent", "list"}
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
		getWorkingDir:  fixedWorkingDir(repo),
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
		getWorkingDir:  fixedWorkingDir(repo),
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
		getWorkingDir:  fixedWorkingDir("/"),
	}

	exitCode, err := exec_.Run(context.Background(), []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if err == nil || !strings.Contains(err.Error(), "could not derive project name") {
		t.Fatalf("expected actionable project-name error, got %v", err)
	}
}

func TestExecutorRun_WorkingDirOutsideRepo_IsActionable(t *testing.T) {
	repo := fakeHexaiRepoDir(t)
	exec_ := Executor{
		commandName:    "ask",
		findBinary:     func() (string, error) { return "/usr/bin/task", nil },
		detectRepoRoot: func(context.Context) (string, error) { return repo, nil },
		getWorkingDir:  fixedWorkingDir(t.TempDir()),
		runCommand: func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
			t.Fatal("runCommand should not be called when cwd is outside repo")
			return nil
		},
	}

	exitCode, err := exec_.Run(context.Background(), []string{"list"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if err == nil || !strings.Contains(err.Error(), "outside git root") {
		t.Fatalf("expected outside-git-root error, got %v", err)
	}
}
