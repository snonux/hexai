package taskproxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

type binaryFinder func() (string, error)

type repoTopLevelDetector func(context.Context) (string, error)

type commandRunner func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error

type Runner struct {
	CommandName    string
	findTaskBinary binaryFinder
	detectRepoRoot repoTopLevelDetector
	runCommand     commandRunner
}

func NewRunner(commandName string) Runner {
	return Runner{
		CommandName:    strings.TrimSpace(commandName),
		findTaskBinary: findTaskBinary,
		detectRepoRoot: detectRepoRoot,
		runCommand:     runTaskCommand,
	}
}

func (r Runner) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	runner := normalizeRunner(r)
	taskPath, err := runner.findTaskBinary()
	if err != nil {
		return 1, fmt.Errorf("%s: Taskwarrior binary lookup failed: %w", runner.commandLabel(), err)
	}
	repoRoot, err := runner.detectRepoRoot(ctx)
	if err != nil {
		return 1, fmt.Errorf("%s: must be run inside a git repository so project:<repo> can be derived: %w", runner.commandLabel(), err)
	}
	taskArgs, err := runner.taskArgs(repoRoot, args)
	if err != nil {
		return 1, fmt.Errorf("%s: %w", runner.commandLabel(), err)
	}
	if err := runner.runCommand(ctx, taskPath, taskArgs, stdin, stdout, stderr); err != nil {
		return runner.exitCodeFor(err)
	}
	return 0, nil
}

func normalizeRunner(r Runner) Runner {
	if r.CommandName == "" {
		r.CommandName = "task"
	}
	if r.findTaskBinary == nil {
		r.findTaskBinary = findTaskBinary
	}
	if r.detectRepoRoot == nil {
		r.detectRepoRoot = detectRepoRoot
	}
	if r.runCommand == nil {
		r.runCommand = runTaskCommand
	}
	return r
}

func (r Runner) commandLabel() string {
	label := strings.TrimSpace(r.CommandName)
	if label == "" {
		return "task"
	}
	return label
}

func (r Runner) taskArgs(repoRoot string, args []string) ([]string, error) {
	projectName, err := projectNameFromRoot(repoRoot)
	if err != nil {
		return nil, err
	}
	return append([]string{"project:" + projectName, "+agent"}, args...), nil
}

func (r Runner) exitCodeFor(err error) (int, error) {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 1, fmt.Errorf("%s: failed to run Taskwarrior: %w", r.commandLabel(), err)
}

func projectNameFromRoot(repoRoot string) (string, error) {
	projectName := filepath.Base(strings.TrimSpace(repoRoot))
	if projectName == "" || projectName == "." || projectName == string(filepath.Separator) {
		return "", fmt.Errorf("could not derive project name from git root %q", repoRoot)
	}
	return projectName, nil
}

func findTaskBinary() (string, error) {
	path, err := exec.LookPath("task")
	if err != nil {
		return "", fmt.Errorf("Taskwarrior binary 'task' not found in PATH; install Taskwarrior and retry")
	}
	return path, nil
}

func detectRepoRoot(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("must be run inside a git repository so project:<repo> can be derived")
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
