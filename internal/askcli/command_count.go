package askcli

import (
	"context"
	"fmt"
	"io"
)

// handleCount counts pending tasks in the executor's project and agent scope.
// Unlike list, it applies no due-date window.
func (d *Dispatcher) handleCount(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: ask count")
		return 1, nil
	}
	return d.runner.Run(ctx, []string{"status:pending", "count"}, nil, stdout, stderr)
}
