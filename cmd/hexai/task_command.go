package main

import (
	"context"
	"io"

	"codeberg.org/snonux/hexai/internal/taskproxy"
)

func runTaskSubcommandIfRequested(args []string, stdin io.Reader, stdout, stderr io.Writer) (bool, int, error) {
	if len(args) == 0 || args[0] != "task" {
		return false, 0, nil
	}
	code, err := taskproxy.NewRunner("hexai task").Run(context.Background(), args[1:], stdin, stdout, stderr)
	return true, code, err
}
