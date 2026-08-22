package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/snonux/hexai/internal/askcli"
)

// dispatcher is the minimal interface runMain depends on; it matches
// (*askcli.Dispatcher).Dispatch so a real dispatcher satisfies it directly.
type dispatcher interface {
	Dispatch(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error)
}

type app struct {
	newDispatcher func() dispatcher
}

func newApp() *app {
	return &app{newDispatcher: func() dispatcher { return askcli.NewDispatcher(nil) }}
}

func main() { os.Exit(newApp().runMain(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

// runMain dispatches the command and returns the process exit code; errors
// are printed to stderr. The dispatcher's exit code is returned regardless
// of err so callers see Taskwarrior's own exit code on failure paths.
func (a *app) runMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	code, err := a.newDispatcher().Dispatch(context.Background(), args, stdin, stdout, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
	}
	return code
}
