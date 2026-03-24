package askcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

func (d Dispatcher) handleUrgency(ctx context.Context, stdout, stderr io.Writer) (int, error) {
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, []string{"export"}, nil, &outBuf, stderr)
	if code != 0 {
		return code, err
	}
	tasks, err := ParseTaskExport(&outBuf)
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to parse task data: %v\n", err)
		return 1, nil
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Urgency > tasks[j].Urgency
	})
	if d.jsonOutput {
		data, err := json.Marshal(tasks)
		if err != nil {
			fmt.Fprintf(stderr, "error: failed to marshal JSON: %v\n", err)
			return 1, nil
		}
		stdout.Write(data)
		io.WriteString(stdout, "\n")
	} else {
		io.WriteString(stdout, FormatTaskList(tasks))
	}
	return 0, nil
}
