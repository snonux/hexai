package askcli

import (
	"bytes"
	"context"
	"io"
	"strings"
)

// taskStep is one Taskwarrior invocation of a write command. Commands made of
// several steps (annotate then done, or stop + tag + annotate for park) let an
// agent do in one call what would otherwise take two or three.
type taskStep struct {
	args func(resolvedTaskSelector) []string
	// optional steps may fail without aborting the command, e.g. stopping a
	// task that was never started.
	optional bool
}

func (d *Dispatcher) runSingleTaskCommand(
	ctx context.Context,
	selector string,
	stdout, stderr io.Writer,
	buildArgs func(resolvedTaskSelector) []string,
) (int, error) {
	return d.runTaskSteps(ctx, selector, stdout, stderr, []taskStep{{args: buildArgs}})
}

// runTaskSteps resolves the selector once, runs the steps in order, and
// prints "ok <id>" when every required step succeeded.
func (d *Dispatcher) runTaskSteps(ctx context.Context, selector string, stdout, stderr io.Writer, steps []taskStep) (int, error) {
	resolved, _, code, err := d.resolveTaskSelector(ctx, selector, stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	for _, step := range steps {
		var outBuf bytes.Buffer
		code, err = d.runner.Run(ctx, step.args(resolved), nil, &outBuf, io.Discard)
		if code != 0 && !step.optional {
			return code, err
		}
	}
	_, _ = io.WriteString(stdout, FormatSuccess(displayResolvedTaskID(resolved)))
	return 0, nil
}

// withOptionalNote returns the steps for a state change that may carry a
// closing note: the note is recorded first, so it is never lost if the state
// change fails, and the task history reads in order.
func withOptionalNote(noteArgs []string, action string) []taskStep {
	var steps []taskStep
	if note := strings.TrimSpace(strings.Join(noteArgs, " ")); note != "" {
		steps = append(steps, taskStep{args: func(resolved resolvedTaskSelector) []string {
			return []string{"uuid:" + resolved.UUID, "annotate", note}
		}})
	}
	return append(steps, taskStep{args: func(resolved resolvedTaskSelector) []string {
		return []string{"uuid:" + resolved.UUID, action}
	}})
}

func (d *Dispatcher) handleDenotate(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		_, _ = io.WriteString(stderr, "error: ask denotate requires an ID or UUID and text argument\n")
		return 1, nil
	}
	text := args[2]
	return d.runSingleTaskCommand(ctx, args[1], stdout, stderr, func(resolved resolvedTaskSelector) []string {
		return []string{"uuid:" + resolved.UUID, "denotate", text}
	})
}

func (d *Dispatcher) handleModify(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		_, _ = io.WriteString(stderr, "error: ask modify requires an ID or UUID and modification args\n")
		return 1, nil
	}
	modArgs := args[2:]
	return d.runSingleTaskCommand(ctx, args[1], stdout, stderr, func(resolved resolvedTaskSelector) []string {
		return append([]string{"uuid:" + resolved.UUID, "modify"}, modArgs...)
	})
}

func (d *Dispatcher) handleAnnotate(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		_, _ = io.WriteString(stderr, "error: ask annotate requires an ID or UUID and note argument\n")
		return 1, nil
	}
	note := strings.Join(args[2:], " ")
	return d.runSingleTaskCommand(ctx, args[1], stdout, stderr, func(resolved resolvedTaskSelector) []string {
		return []string{"uuid:" + resolved.UUID, "annotate", note}
	})
}

func (d *Dispatcher) handleStart(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		_, _ = io.WriteString(stderr, "error: ask start requires an ID or UUID argument\n")
		return 1, nil
	}
	resolved, tasks, code, err := d.resolveTaskSelector(ctx, args[1], stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	if code != 0 {
		return code, nil
	}
	if depCode := d.verifyDependenciesCompletedForStart(ctx, tasks[0], stderr); depCode != 0 {
		return depCode, nil
	}

	var outBuf bytes.Buffer
	code, err = d.runner.Run(ctx, []string{"uuid:" + resolved.UUID, "start"}, nil, &outBuf, io.Discard)
	if code != 0 {
		return code, err
	}
	_, _ = io.WriteString(stdout, FormatSuccess(displayResolvedTaskID(resolved)))
	return 0, nil
}

func (d *Dispatcher) handleStop(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		_, _ = io.WriteString(stderr, "error: ask stop requires an ID or UUID argument\n")
		return 1, nil
	}
	// ask stop <id> ["note"]: the note is the handoff for whoever resumes.
	return d.runTaskSteps(ctx, args[1], stdout, stderr, withOptionalNote(args[2:], "stop"))
}

func (d *Dispatcher) handleDone(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		_, _ = io.WriteString(stderr, "error: ask done requires an ID or UUID argument\n")
		return 1, nil
	}
	// ask done <id> ["note"]: the note is the delivery summary.
	return d.runTaskSteps(ctx, args[1], stdout, stderr, withOptionalNote(args[2:], "done"))
}

func (d *Dispatcher) handlePriority(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		_, _ = io.WriteString(stderr, "error: ask priority requires an ID or UUID and priority (H/M/L)\n")
		return 1, nil
	}
	priority := args[2]
	return d.runSingleTaskCommand(ctx, args[1], stdout, stderr, func(resolved resolvedTaskSelector) []string {
		return []string{"uuid:" + resolved.UUID, "modify", "priority:" + priority}
	})
}

func (d *Dispatcher) handleTag(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 3 {
		_, _ = io.WriteString(stderr, "error: ask tag requires an ID or UUID and +/-tag\n")
		return 1, nil
	}
	tag := args[2]
	return d.runSingleTaskCommand(ctx, args[1], stdout, stderr, func(resolved resolvedTaskSelector) []string {
		return []string{"uuid:" + resolved.UUID, "modify", tag}
	})
}
