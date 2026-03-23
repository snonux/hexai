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
	uuid := NormalizeUUID(args[1])
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
	modifiers, description := parseAddArgs(args[1:])
	var outBuf bytes.Buffer
	// rc.verbose=new-uuid instructs taskwarrior to emit "Created task <uuid>."
	// so we get the UUID directly from the add output without a follow-up export.
	taskArgs := []string{"add", "rc.verbose=new-uuid"}
	taskArgs = append(taskArgs, modifiers...)
	taskArgs = append(taskArgs, description)
	code, err := d.runner.Run(ctx, taskArgs, nil, &outBuf, stderr)
	if code != 0 {
		return code, err
	}
	uuid := extractUUIDFromAddOutput(outBuf.String())
	if uuid == "" {
		io.WriteString(stderr, "error: could not parse UUID from task creation output\n")
		return 1, nil
	}
	io.WriteString(stdout, uuid+"\n")
	return 0, nil
}

// extractUUIDFromAddOutput parses the UUID from taskwarrior's
// "Created task <uuid>." output (produced when rc.verbose=new-uuid is set).
func extractUUIDFromAddOutput(output string) string {
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.HasPrefix(line, "Created task ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return strings.TrimSuffix(parts[2], ".")
			}
		}
	}
	return ""
}

// parseAddArgs splits args into taskwarrior modifier tokens and the description.
// Modifier tokens are args that start with "priority:", "+", or "-" AND contain
// no spaces (tags and priority flags cannot have spaces). The first arg that is
// not a modifier begins the description; all remaining args are joined with spaces.
// If all args are modifiers the description is empty.
func parseAddArgs(args []string) (modifiers []string, description string) {
	for i, arg := range args {
		isModifier := !strings.Contains(arg, " ") &&
			(strings.HasPrefix(arg, "priority:") || strings.HasPrefix(arg, "+") || strings.HasPrefix(arg, "-"))
		if isModifier {
			modifiers = append(modifiers, arg)
		} else {
			description = strings.Join(args[i:], " ")
			return
		}
	}
	// All args were modifiers; no description provided.
	return
}
