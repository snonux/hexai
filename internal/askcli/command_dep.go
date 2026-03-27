package askcli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

func (d Dispatcher) handleDep(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		io.WriteString(stderr, "error: ask dep requires an operation (add/rm/list) and arguments\n")
		return 1, nil
	}
	op := args[1]
	switch op {
	case "add", "rm":
		return d.handleDepAddRm(ctx, args, stdout, stderr)
	case "list":
		return d.handleDepList(ctx, args, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: ask dep: unknown operation %q (use add, rm, or list)\n", op)
		return 1, nil
	}
}

func (d Dispatcher) handleDepAddRm(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 4 {
		io.WriteString(stderr, "error: ask dep add/rm requires <id|uuid> <dep-id|dep-uuid>\n")
		return 1, nil
	}
	resolved, _, code, err := d.resolveTaskSelector(ctx, args[2], stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	dependency, _, code, err := d.resolveTaskSelector(ctx, args[3], stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	op := args[1]
	var modArg string
	if op == "add" {
		modArg = "depends:" + dependency.UUID
	} else {
		modArg = "depends:-" + dependency.UUID
	}
	var outBuf bytes.Buffer
	// uuid:<uuid> scopes the modify to exactly one task; modArg sets the dependency.
	code, err = d.runner.Run(ctx, []string{"uuid:" + resolved.UUID, "modify", modArg}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(displayResolvedTaskID(resolved)))
	return 0, nil
}

func (d Dispatcher) handleDepList(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		io.WriteString(stderr, "error: ask dep list requires <id|uuid>\n")
		return 1, nil
	}
	_, tasks, code, err := d.resolveTaskSelector(ctx, args[2], stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	if len(tasks) == 0 {
		io.WriteString(stdout, "no dependencies\n")
		return 0, nil
	}
	task := tasks[0]
	if len(task.Depends) == 0 {
		io.WriteString(stdout, "no dependencies\n")
	} else {
		aliases, err := ensureTaskAliasesForUUIDs(task.Depends)
		if err != nil {
			fmt.Fprintf(stderr, "error: failed to load task aliases: %v\n", err)
			return 1, nil
		}
		ids := make([]string, 0, len(task.Depends))
		for _, uuid := range task.Depends {
			ids = append(ids, displayTaskAlias(uuid, aliases))
		}
		io.WriteString(stdout, strings.Join(ids, "\n")+"\n")
	}
	return 0, nil
}
