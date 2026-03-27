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
	aliases, err := ensureTaskAliases(tasks)
	if err != nil {
		fmt.Fprintf(stderr, "warning: failed to update task alias cache: %v\n", err)
		aliases = nil
	}
	for _, selector := range taskCompletionSelectors(tasks, aliases) {
		_, _ = io.WriteString(stdout, selector+"\n")
	}
	return 0, nil
}

func taskCompletionSelectors(tasks []TaskExport, aliases map[string]string) []string {
	selectors := make([]string, 0, len(tasks)*2)
	for _, task := range tasks {
		if task.UUID == "" {
			continue
		}
		if alias := displayTaskAlias(task.UUID, aliases); alias != "" && alias != task.UUID {
			selectors = append(selectors, alias)
		}
		selectors = append(selectors, task.UUID)
	}
	return selectors
}
