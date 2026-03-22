package askcli

import (
	"bytes"
	"context"
	"io"
	"strings"
)

func (d Dispatcher) handleDenotate(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		io.WriteString(stderr, "error: ask denotate requires a UUID and text argument\n")
		return 1, nil
	}
	uuid := args[1]
	if IsNumericID(uuid) {
		io.WriteString(stderr, RejectNumericID())
		return 1, nil
	}
	text := args[2]
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, []string{"uuid:" + uuid, "denotate", text}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(uuid))
	return 0, nil
}

func (d Dispatcher) handleModify(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		io.WriteString(stderr, "error: ask modify requires a UUID and modification args\n")
		return 1, nil
	}
	uuid := args[1]
	if IsNumericID(uuid) {
		io.WriteString(stderr, RejectNumericID())
		return 1, nil
	}
	modArgs := args[2:]
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, append([]string{"modify", uuid}, modArgs...), nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(uuid))
	return 0, nil
}

func (d Dispatcher) handleAnnotate(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		io.WriteString(stderr, "error: ask annotate requires a UUID and note argument\n")
		return 1, nil
	}
	uuid := args[1]
	if IsNumericID(uuid) {
		io.WriteString(stderr, RejectNumericID())
		return 1, nil
	}
	note := strings.Join(args[2:], " ")
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, []string{"annotate", uuid, note}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(uuid))
	return 0, nil
}

func (d Dispatcher) handleStart(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		io.WriteString(stderr, "error: ask start requires a UUID argument\n")
		return 1, nil
	}
	uuid := args[1]
	if IsNumericID(uuid) {
		io.WriteString(stderr, RejectNumericID())
		return 1, nil
	}
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, []string{"start", uuid}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(uuid))
	return 0, nil
}

func (d Dispatcher) handleStop(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		io.WriteString(stderr, "error: ask stop requires a UUID argument\n")
		return 1, nil
	}
	uuid := args[1]
	if IsNumericID(uuid) {
		io.WriteString(stderr, RejectNumericID())
		return 1, nil
	}
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, []string{"stop", uuid}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(uuid))
	return 0, nil
}

func (d Dispatcher) handleDone(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		io.WriteString(stderr, "error: ask done requires a UUID argument\n")
		return 1, nil
	}
	uuid := args[1]
	if IsNumericID(uuid) {
		io.WriteString(stderr, RejectNumericID())
		return 1, nil
	}
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, []string{"done", uuid}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(uuid))
	return 0, nil
}

func (d Dispatcher) handlePriority(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		io.WriteString(stderr, "error: ask priority requires a UUID and priority (H/M/L)\n")
		return 1, nil
	}
	uuid := args[1]
	if IsNumericID(uuid) {
		io.WriteString(stderr, RejectNumericID())
		return 1, nil
	}
	priority := args[2]
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, []string{"uuid:" + uuid, "modify", "priority:" + priority}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(uuid))
	return 0, nil
}

func (d Dispatcher) handleTag(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		io.WriteString(stderr, "error: ask tag requires a UUID and +/-tag\n")
		return 1, nil
	}
	uuid := args[1]
	if IsNumericID(uuid) {
		io.WriteString(stderr, RejectNumericID())
		return 1, nil
	}
	tag := args[2]
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, []string{"uuid:" + uuid, "modify", tag}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(uuid))
	return 0, nil
}
