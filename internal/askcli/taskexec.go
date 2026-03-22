package askcli

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

type Executor struct {
	commandName    string
	findBinary     binaryFinder
	detectRepoRoot repoTopLevelDetector
	runCommand     commandRunner
}

func NewExecutor(commandName string) Executor {
	return Executor{
		commandName:    strings.TrimSpace(commandName),
		findBinary:     findTaskBinary,
		detectRepoRoot: detectRepoRoot,
		runCommand:     runTaskCommand,
	}
}

func (e Executor) taskArgs(repoRoot string, args []string) ([]string, error) {
	projectName, err := projectNameFromRoot(repoRoot)
	if err != nil {
		return nil, err
	}
	return append([]string{"project:" + projectName, "+agent"}, args...), nil
}

func (e Executor) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	executor := normalizeExecutor(e)
	taskPath, err := executor.findBinary()
	if err != nil {
		return 1, fmt.Errorf("%s: task binary lookup failed: %w", executor.label(), err)
	}
	repoRoot, err := executor.detectRepoRoot(ctx)
	if err != nil {
		return 1, fmt.Errorf("%s: must be run inside a git repository: %w", executor.label(), err)
	}
	taskArgs, err := executor.taskArgs(repoRoot, args)
	if err != nil {
		return 1, fmt.Errorf("%s: %w", executor.label(), err)
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
	if e.runCommand == nil {
		e.runCommand = runTaskCommand
	}
	return e
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
