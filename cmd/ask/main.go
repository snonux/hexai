package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"codeberg.org/snonux/hexai/internal/askcli"
)

// dispatcher is the minimal interface runMain depends on; it matches
// (*askcli.Dispatcher).Dispatch so a real dispatcher satisfies it directly.
type dispatcher interface {
	Dispatch(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error)
}

// dispatcherFactory is a test seam: override to inject a fake dispatcher so
// runMain can be exercised without a real `task` binary on PATH.
var dispatcherFactory = func() dispatcher {
	return askcli.NewDispatcher(nil)
}

func main() { os.Exit(runMain(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

// runMain dispatches the command and returns the process exit code; errors
// are printed to stderr. The dispatcher's exit code is returned regardless
// of err so callers see Taskwarrior's own exit code on failure paths.
func runMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	code, err := dispatcherFactory().Dispatch(context.Background(), args, stdin, stdout, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
	}
	return code
}
