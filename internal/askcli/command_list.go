package askcli

import (
	"bytes"
	"context"
	"io"
	"sort"
	"strings"
)

func (d Dispatcher) handleList(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	filterArgs := []string{"export"}
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "limit:") || strings.HasPrefix(arg, "sort:") ||
			strings.HasPrefix(arg, "+") || arg == "started" {
			filterArgs = append(filterArgs, arg)
		}
	}
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, filterArgs, nil, &outBuf, stderr)
	if code != 0 {
		return code, err
	}
	tasks, err := ParseTaskExport(&outBuf)
	if err != nil {
		return 1, nil
	}
	sort.Slice(tasks, func(i, j int) bool {
		pi := priorityOrder(tasks[i].Priority)
		pj := priorityOrder(tasks[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return tasks[i].Urgency > tasks[j].Urgency
	})
	io.WriteString(stdout, FormatTaskList(tasks))
	return 0, nil
}

func priorityOrder(p string) int {
	switch p {
	case "H":
		return 1
	case "M":
		return 2
	case "L":
		return 3
	default:
		return 4
	}
}
