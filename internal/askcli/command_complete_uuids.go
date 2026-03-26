package askcli

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

func (d Dispatcher) handleCompleteUUIDs(ctx context.Context, stdout, stderr io.Writer) (int, error) {
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, []string{"status:pending", "export"}, nil, &outBuf, stderr)
	if code != 0 {
		return code, err
	}
	tasks, err := ParseTaskExport(&outBuf)
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to parse task data: %v\n", err)
		return 1, nil
	}
	if _, err := ensureTaskAliases(tasks); err != nil {
		fmt.Fprintf(stderr, "warning: failed to update task alias cache: %v\n", err)
	}
	for _, task := range tasks {
		if task.UUID == "" {
			continue
		}
		_, _ = io.WriteString(stdout, task.UUID+"\n")
	}
	return 0, nil
}
