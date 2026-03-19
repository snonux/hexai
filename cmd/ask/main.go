package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"codeberg.org/snonux/hexai/internal/taskproxy"
)

type taskRunner func(context.Context, []string, io.Reader, io.Writer, io.Writer) (int, error)

type askRunner struct {
	runTask taskRunner
}

func newAskRunner() askRunner {
	return askRunner{runTask: runAskTask}
}

func main() {
	if exitCode := newAskRunner().run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); exitCode != 0 {
		os.Exit(exitCode)
	}
}

func (r askRunner) run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	runner := normalizeAskRunner(r)
	exitCode, err := runner.runTask(context.Background(), args, stdin, stdout, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
	}
	return exitCode
}

func normalizeAskRunner(r askRunner) askRunner {
	if r.runTask == nil {
		r.runTask = runAskTask
	}
	return r
}

func runAskTask(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return taskproxy.NewRunner("ask").Run(ctx, args, stdin, stdout, stderr)
}
