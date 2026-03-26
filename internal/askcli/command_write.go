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
	resolved, _, code, err := d.resolveTaskSelector(ctx, args[1], stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	text := args[2]
	var outBuf bytes.Buffer
	code, err = d.runner.Run(ctx, []string{"uuid:" + resolved.UUID, "denotate", text}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(resolved.UUID))
	return 0, nil
}

func (d Dispatcher) handleModify(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		io.WriteString(stderr, "error: ask modify requires a UUID and modification args\n")
		return 1, nil
	}
	resolved, _, code, err := d.resolveTaskSelector(ctx, args[1], stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	modArgs := args[2:]
	var outBuf bytes.Buffer
	code, err = d.runner.Run(ctx, append([]string{"uuid:" + resolved.UUID, "modify"}, modArgs...), nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(resolved.UUID))
	return 0, nil
}

func (d Dispatcher) handleAnnotate(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		io.WriteString(stderr, "error: ask annotate requires a UUID and note argument\n")
		return 1, nil
	}
	resolved, _, code, err := d.resolveTaskSelector(ctx, args[1], stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	note := strings.Join(args[2:], " ")
	var outBuf bytes.Buffer
	code, err = d.runner.Run(ctx, []string{"uuid:" + resolved.UUID, "annotate", note}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(resolved.UUID))
	return 0, nil
}

func (d Dispatcher) handleStart(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		io.WriteString(stderr, "error: ask start requires a UUID argument\n")
		return 1, nil
	}
	resolved, _, code, err := d.resolveTaskSelector(ctx, args[1], stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	var outBuf bytes.Buffer
	// uuid:<uuid> is used as the filter so taskwarrior selects the exact task;
	// the action verb follows the filter.
	code, err = d.runner.Run(ctx, []string{"uuid:" + resolved.UUID, "start"}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(resolved.UUID))
	return 0, nil
}

func (d Dispatcher) handleStop(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		io.WriteString(stderr, "error: ask stop requires a UUID argument\n")
		return 1, nil
	}
	resolved, _, code, err := d.resolveTaskSelector(ctx, args[1], stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	var outBuf bytes.Buffer
	code, err = d.runner.Run(ctx, []string{"uuid:" + resolved.UUID, "stop"}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(resolved.UUID))
	return 0, nil
}

func (d Dispatcher) handleDone(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		io.WriteString(stderr, "error: ask done requires a UUID argument\n")
		return 1, nil
	}
	resolved, _, code, err := d.resolveTaskSelector(ctx, args[1], stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	var outBuf bytes.Buffer
	code, err = d.runner.Run(ctx, []string{"uuid:" + resolved.UUID, "done"}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(resolved.UUID))
	return 0, nil
}

func (d Dispatcher) handlePriority(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		io.WriteString(stderr, "error: ask priority requires a UUID and priority (H/M/L)\n")
		return 1, nil
	}
	resolved, _, code, err := d.resolveTaskSelector(ctx, args[1], stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	priority := args[2]
	var outBuf bytes.Buffer
	code, err = d.runner.Run(ctx, []string{"uuid:" + resolved.UUID, "modify", "priority:" + priority}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(resolved.UUID))
	return 0, nil
}

func (d Dispatcher) handleTag(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		io.WriteString(stderr, "error: ask tag requires a UUID and +/-tag\n")
		return 1, nil
	}
	resolved, _, code, err := d.resolveTaskSelector(ctx, args[1], stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	tag := args[2]
	var outBuf bytes.Buffer
	code, err = d.runner.Run(ctx, []string{"uuid:" + resolved.UUID, "modify", tag}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	io.WriteString(stdout, FormatSuccess(resolved.UUID))
	return 0, nil
}
