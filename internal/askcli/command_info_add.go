package askcli

import (
	"bytes"
	"context"
	"io"
	"strings"
)

func (d Dispatcher) handleInfo(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		io.WriteString(stderr, "error: ask info requires a UUID argument\n")
		return 1, nil
	}
	uuid := args[1]
	if IsNumericID(uuid) {
		io.WriteString(stderr, RejectNumericID())
		return 1, nil
	}
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, []string{"uuid", uuid, "export"}, nil, &outBuf, stderr)
	if code != 0 {
		return code, err
	}
	tasks, err := ParseTaskExport(&outBuf)
	if err != nil || len(tasks) == 0 {
		io.WriteString(stderr, "error: task not found\n")
		return 1, nil
	}
	io.WriteString(stdout, FormatTaskInfo(tasks[0]))
	return 0, nil
}

func (d Dispatcher) handleAdd(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		io.WriteString(stderr, "error: ask add requires a description\n")
		return 1, nil
	}
	description := strings.Join(args[1:], " ")
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, []string{"add", description}, nil, &outBuf, stderr)
	if code != 0 {
		return code, err
	}
	createdUUID := ExtractUUIDFromOutput(outBuf.String())
	if createdUUID == "" {
		io.WriteString(stderr, "error: could not extract UUID from task creation output\n")
		return 1, nil
	}
	io.WriteString(stdout, createdUUID+"\n")
	return 0, nil
}
