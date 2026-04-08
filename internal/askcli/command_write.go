package askcli

import (
	"bytes"
	"context"
	"io"
	"strings"
)

func (d *Dispatcher) runSingleTaskCommand(
	ctx context.Context,
	selector string,
	stdout, stderr io.Writer,
	buildArgs func(resolvedTaskSelector) []string,
) (int, error) {
	resolved, _, code, err := d.resolveTaskSelector(ctx, selector, stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}

	var outBuf bytes.Buffer
	code, err = d.runner.Run(ctx, buildArgs(resolved), nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}

	_, _ = io.WriteString(stdout, FormatSuccess(displayResolvedTaskID(resolved)))
	return 0, nil
}

func (d *Dispatcher) handleDenotate(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		_, _ = io.WriteString(stderr, "error: do denotate requires an ID or UUID and text argument\n")
		return 1, nil
	}
	text := args[2]
	return d.runSingleTaskCommand(ctx, args[1], stdout, stderr, func(resolved resolvedTaskSelector) []string {
		return []string{"uuid:" + resolved.UUID, "denotate", text}
	})
}

func (d *Dispatcher) handleModify(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		_, _ = io.WriteString(stderr, "error: do modify requires an ID or UUID and modification args\n")
		return 1, nil
	}
	modArgs := args[2:]
	return d.runSingleTaskCommand(ctx, args[1], stdout, stderr, func(resolved resolvedTaskSelector) []string {
		return append([]string{"uuid:" + resolved.UUID, "modify"}, modArgs...)
	})
}

func (d *Dispatcher) handleAnnotate(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		_, _ = io.WriteString(stderr, "error: do annotate requires an ID or UUID and note argument\n")
		return 1, nil
	}
	note := strings.Join(args[2:], " ")
	return d.runSingleTaskCommand(ctx, args[1], stdout, stderr, func(resolved resolvedTaskSelector) []string {
		return []string{"uuid:" + resolved.UUID, "annotate", note}
	})
}

func (d *Dispatcher) handleStart(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		_, _ = io.WriteString(stderr, "error: do start requires an ID or UUID argument\n")
		return 1, nil
	}
	return d.runSingleTaskCommand(ctx, args[1], stdout, stderr, func(resolved resolvedTaskSelector) []string {
		// uuid:<uuid> is used as the filter so taskwarrior selects the exact task;
		// the action verb follows the filter.
		return []string{"uuid:" + resolved.UUID, "start"}
	})
}

func (d *Dispatcher) handleStop(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		_, _ = io.WriteString(stderr, "error: do stop requires an ID or UUID argument\n")
		return 1, nil
	}
	return d.runSingleTaskCommand(ctx, args[1], stdout, stderr, func(resolved resolvedTaskSelector) []string {
		return []string{"uuid:" + resolved.UUID, "stop"}
	})
}

func (d *Dispatcher) handleDone(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		_, _ = io.WriteString(stderr, "error: do done requires an ID or UUID argument\n")
		return 1, nil
	}
	return d.runSingleTaskCommand(ctx, args[1], stdout, stderr, func(resolved resolvedTaskSelector) []string {
		return []string{"uuid:" + resolved.UUID, "done"}
	})
}

func (d *Dispatcher) handlePriority(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		_, _ = io.WriteString(stderr, "error: do priority requires an ID or UUID and priority (H/M/L)\n")
		return 1, nil
	}
	priority := args[2]
	return d.runSingleTaskCommand(ctx, args[1], stdout, stderr, func(resolved resolvedTaskSelector) []string {
		return []string{"uuid:" + resolved.UUID, "modify", "priority:" + priority}
	})
}

func (d *Dispatcher) handleTag(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		_, _ = io.WriteString(stderr, "error: do tag requires an ID or UUID and +/-tag\n")
		return 1, nil
	}
	tag := args[2]
	return d.runSingleTaskCommand(ctx, args[1], stdout, stderr, func(resolved resolvedTaskSelector) []string {
		return []string{"uuid:" + resolved.UUID, "modify", tag}
	})
}
