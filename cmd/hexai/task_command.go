package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

type taskBinaryFinder func() (string, error)

type repoTopLevelDetector func(context.Context) (string, error)

type taskCommandRunner func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error

type taskRunner struct {
	findTaskBinary taskBinaryFinder
	detectRepoRoot repoTopLevelDetector
	runCommand     taskCommandRunner
}

func newTaskRunner() taskRunner {
	return taskRunner{
		findTaskBinary: findTaskBinary,
		detectRepoRoot: detectRepoRoot,
		runCommand:     runTaskCommand,
	}
}

func runTaskSubcommandIfRequested(args []string, stdin io.Reader, stdout, stderr io.Writer) (bool, int, error) {
	if len(args) == 0 || args[0] != "task" {
		return false, 0, nil
	}
	code, err := newTaskRunner().run(context.Background(), args[1:], stdin, stdout, stderr)
	return true, code, err
}

func (r taskRunner) run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	runner := normalizeTaskRunner(r)
	taskPath, err := runner.findTaskBinary()
	if err != nil {
		return 1, fmt.Errorf("hexai task: Taskwarrior binary lookup failed: %w", err)
	}
	repoRoot, err := runner.detectRepoRoot(ctx)
	if err != nil {
		return 1, fmt.Errorf("hexai task: must be run inside a git repository so project:<repo> can be derived: %w", err)
	}
	projectName := filepath.Base(strings.TrimSpace(repoRoot))
	if projectName == "" || projectName == "." || projectName == string(filepath.Separator) {
		return 1, fmt.Errorf("hexai task: could not derive project name from git root %q", repoRoot)
	}
	taskArgs := append([]string{"project:" + projectName, "+agent"}, args...)
	if err := runner.runCommand(ctx, taskPath, taskArgs, stdin, stdout, stderr); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("hexai task: failed to run Taskwarrior: %w", err)
	}
	return 0, nil
}

func normalizeTaskRunner(r taskRunner) taskRunner {
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
