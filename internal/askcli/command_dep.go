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
		io.WriteString(stderr, "error: ask dep add/rm requires <uuid> <dep-uuid>\n")
		return 1, nil
	}
	uuid := args[2]
	if IsNumericID(uuid) {
		io.WriteString(stderr, RejectNumericID())
		return 1, nil
	}
	depUUID := args[3]
	if IsNumericID(depUUID) {
		io.WriteString(stderr, RejectNumericID())
		return 1, nil
	}
	op := args[1]
	var modArg string
	if op == "add" {
		modArg = "depends:" + depUUID
	} else {
		modArg = "depends:-" + depUUID
	}
	var outBuf bytes.Buffer
	// uuid:<uuid> scopes the modify to exactly one task; modArg sets the dependency.
	code, err := d.runner.Run(ctx, []string{"uuid:" + uuid, "modify", modArg}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(uuid))
	return 0, nil
}

func (d Dispatcher) handleDepList(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		io.WriteString(stderr, "error: ask dep list requires <uuid>\n")
		return 1, nil
	}
	uuid := args[2]
	if IsNumericID(uuid) {
		io.WriteString(stderr, RejectNumericID())
		return 1, nil
	}
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, []string{"uuid:" + uuid, "export"}, nil, &outBuf, stderr)
	if code != 0 {
		return code, err
	}
	tasks, err := ParseTaskExport(&outBuf)
	if err != nil {
		return 1, nil
	}
	if len(tasks) == 0 {
		io.WriteString(stdout, "no dependencies\n")
		return 0, nil
	}
	task := tasks[0]
	if len(task.Depends) == 0 {
		io.WriteString(stdout, "no dependencies\n")
	} else {
		io.WriteString(stdout, strings.Join(task.Depends, "\n")+"\n")
	}
	return 0, nil
}
