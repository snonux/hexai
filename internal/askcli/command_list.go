package askcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

func (d Dispatcher) handleList(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	return d.handleListWithFilters(ctx, []string{"status:pending"}, args[1:], stdout, stderr)
}

func (d Dispatcher) handleAll(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	return d.handleListWithFilters(ctx, nil, args[1:], stdout, stderr)
}

func (d Dispatcher) handleReady(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	return d.handleListWithFilters(ctx, []string{"+READY"}, args[1:], stdout, stderr)
}

// handleListWithFilters is the shared implementation for list/all/ready.
// initialFilters seeds the taskwarrior filter; extraArgs are user-supplied
// filter modifiers (limit:, sort:, +tag, started).
func (d Dispatcher) handleListWithFilters(ctx context.Context, initialFilters, extraArgs []string, stdout, stderr io.Writer) (int, error) {
	filterArgs := append([]string(nil), initialFilters...)
	for _, arg := range extraArgs {
		if strings.HasPrefix(arg, "limit:") || strings.HasPrefix(arg, "sort:") ||
			strings.HasPrefix(arg, "+") || arg == "started" {
			filterArgs = append(filterArgs, arg)
		}
	}
	filterArgs = append(filterArgs, "export")
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, filterArgs, nil, &outBuf, stderr)
	if code != 0 {
		return code, err
	}
	tasks, err := ParseTaskExport(&outBuf)
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to parse task data: %v\n", err)
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
	if d.jsonOutput {
		data, err := json.Marshal(tasks)
		if err != nil {
			fmt.Fprintf(stderr, "error: failed to marshal JSON: %v\n", err)
			return 1, nil
		}
		stdout.Write(data)
		io.WriteString(stdout, "\n")
	} else {
		aliases, err := ensureTaskAliases(tasks)
		if err != nil {
			fmt.Fprintf(stderr, "error: failed to load task aliases: %v\n", err)
			return 1, nil
		}
		io.WriteString(stdout, FormatTaskListForWidth(tasks, aliases, detectTaskListTerminalWidth(stdout)))
	}
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
