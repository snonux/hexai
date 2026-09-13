package askcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type binaryFinder func() (string, error)

type repoTopLevelDetector func(context.Context) (string, error)

type workingDirectory func() (string, error)

type commandRunner func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error

// Executor encapsulates how the ask CLI communicates with the Taskwarrior binary.
type Executor struct {
	commandName    string
	findBinary     binaryFinder
	detectRepoRoot repoTopLevelDetector
	getWorkingDir  workingDirectory
	runCommand     commandRunner
}

// NewExecutor constructs an Executor that invokes Taskwarrior via the given command name.
func NewExecutor(commandName string) Executor {
	return Executor{
		commandName:    strings.TrimSpace(commandName),
		findBinary:     findTaskBinary,
		detectRepoRoot: detectRepoRoot,
		getWorkingDir:  os.Getwd,
		runCommand:     runTaskCommand,
	}
}

func (e Executor) taskArgs(ctx context.Context, repoRoot string, args []string) ([]string, error) {
	projectName, ok := taskProjectFromContext(ctx)
	if !ok {
		cwd, err := e.workingDir()
		if err != nil {
			return nil, err
		}
		projectName, err = projectNameFromRoot(repoRoot, cwd)
		if err != nil {
			return nil, err
		}
	}
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return nil, fmt.Errorf("project override proj:<name> requires a project name")
	}
	// rc.verbose=nothing suppresses Taskwarrior's configuration override
	// banner, while rc.confirmation=off keeps non-interactive commands from
	// prompting when stdin is unavailable.
	if len(args) > 0 && args[0] == "add" {
		return addTaskArgs(projectName, taskScopeFromContext(ctx), args), nil
	}
	scopeFilter := taskScopeFilter(taskScopeFromContext(ctx))
	return append([]string{"rc.verbose=nothing", "rc.confirmation=off", projectReadFilter(projectName), scopeFilter}, args...), nil
}

func addTaskArgs(projectName string, scope taskScopeMode, args []string) []string {
	taskArgs := []string{"rc.verbose=nothing", "rc.confirmation=off", "project:" + projectName, "add"}
	nextArg := 1
	for nextArg < len(args) && strings.HasPrefix(args[nextArg], "rc.") {
		taskArgs = append(taskArgs, args[nextArg])
		nextArg++
	}
	if scope == taskScopeAgent {
		taskArgs = append(taskArgs, "+agent")
	}
	return append(taskArgs, args[nextArg:]...)
}

// Run delegates CLI arguments to Taskwarrior, enforcing agent defaults and error handling.
func (e Executor) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	executor := normalizeExecutor(e)
	taskPath, err := executor.findBinary()
	if err != nil {
		return 1, fmt.Errorf("%s: task binary lookup failed: %w", executor.label(), err)
	}
	gitRoot, gitErr := executor.detectRepoRoot(ctx)
	repoRoot := ""
	if _, ok := taskProjectFromContext(ctx); !ok {
		if gitErr != nil {
			return 1, fmt.Errorf("%s: must be run inside a git repository: %w", executor.label(), gitErr)
		}
		repoRoot = gitRoot
	}
	taskArgs, err := executor.taskArgs(ctx, repoRoot, args)
	if err != nil {
		return 1, fmt.Errorf("%s: %w", executor.label(), err)
	}
	if gitErr == nil {
		unlockAsk, lerr := acquireAskRepoLock(ctx, gitRoot)
		if lerr != nil {
			return 1, fmt.Errorf("%s: %w", executor.label(), lerr)
		}
		defer func() { _ = unlockAsk() }()
	}
	if err := executor.runCommand(ctx, taskPath, taskArgs, stdin, stdout, stderr); err != nil {
		return exitCodeFor(err), nil
	}
	return 0, nil
}

func (e Executor) label() string {
	label := strings.TrimSpace(e.commandName)
	if label == "" {
		return "ask"
	}
	return label
}

func (e Executor) workingDir() (string, error) {
	if e.getWorkingDir != nil {
		return e.getWorkingDir()
	}
	return os.Getwd()
}

func normalizeExecutor(e Executor) Executor {
	if e.commandName == "" {
		e.commandName = "ask"
	}
	if e.findBinary == nil {
		e.findBinary = findTaskBinary
	}
	if e.detectRepoRoot == nil {
		e.detectRepoRoot = detectRepoRoot
	}
	if e.getWorkingDir == nil {
		e.getWorkingDir = os.Getwd
	}
	if e.runCommand == nil {
		e.runCommand = runTaskCommand
	}
	return e
}

// projectReadFilter matches project P and hierarchical descendants P.* without
// matching siblings like P-other (Taskwarrior's plain project:P is a string prefix).
func projectReadFilter(projectName string) string {
	return "(project.is:" + projectName + " or project:" + projectName + ".)"
}

func projectNameFromRoot(repoRoot, cwd string) (string, error) {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return "", fmt.Errorf("could not derive project name from git root %q", repoRoot)
	}
	repoRoot = canonicalPath(repoRoot)
	projectName := filepath.Base(repoRoot)
	if projectName == "" || projectName == "." || projectName == string(filepath.Separator) {
		return "", fmt.Errorf("could not derive project name from git root %q", repoRoot)
	}
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return "", fmt.Errorf("could not derive project name: working directory is empty")
	}
	cwd = canonicalPath(cwd)
	rel, err := filepath.Rel(repoRoot, cwd)
	if err != nil {
		return "", fmt.Errorf("could not derive project name from cwd %q under git root %q: %w", cwd, repoRoot, err)
	}
	if rel == "." {
		return projectName, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("working directory %q is outside git root %q", cwd, repoRoot)
	}
	hierarchical := strings.ReplaceAll(filepath.ToSlash(rel), "/", ".")
	return projectName + "." + hierarchical, nil
}

// canonicalPath resolves symlinks when possible so logical cwd paths under a
// symlinked checkout still compare equal to git's physical toplevel.
func canonicalPath(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func findTaskBinary() (string, error) {
	path, err := exec.LookPath("task")
	if err != nil {
		return "", fmt.Errorf("task binary 'task' not found in PATH; install task and retry")
	}
	return path, nil
}

func detectRepoRoot(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("must be run inside a git repository so project name can be derived")
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", fmt.Errorf("git returned an empty repository root")
	}
	return root, nil
}

func runTaskCommand(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func exitCodeFor(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}
