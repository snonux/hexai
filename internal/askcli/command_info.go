package askcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// infoLastPrefix is the "ask info" modifier that limits the output to the
// newest N annotations. Long-running tasks collect dozens of annotations, and
// an agent re-reading a task it already knows only needs the recent ones.
const infoLastPrefix = "last:"

// splitInfoLast removes a "last:<n>" modifier from the info arguments and
// returns the remaining arguments and n (0 when the modifier is absent).
func splitInfoLast(args []string) ([]string, int, error) {
	rest := make([]string, 0, len(args))
	last := 0
	for i, arg := range args {
		if i == 0 || !strings.HasPrefix(arg, infoLastPrefix) {
			rest = append(rest, arg)
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(arg, infoLastPrefix))
		if err != nil || n < 1 {
			return nil, 0, fmt.Errorf("%q is not valid; use last:<n> with n >= 1, e.g. ask info <id> last:3", arg)
		}
		last = n
	}
	return rest, last, nil
}

func (d *Dispatcher) handleInfo(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	args, last, err := splitInfoLast(args)
	if err != nil {
		writeInfoError(stderr, err)
		return 1, nil
	}
	tasks, code, err := d.infoTasks(ctx, args, stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	allUUIDs := append([]string{tasks[0].UUID}, tasks[0].Depends...)
	aliases, err := d.aliasCache.withDefaults().ensureTaskAliasesForUUIDs(allUUIDs)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: failed to load task aliases: %v\n", err)
		return 1, nil
	}
	if d.jsonOutput {
		data, err := json.Marshal(withTaskIDs(tasks, aliases))
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "error: failed to marshal JSON: %v\n", err)
			return 1, nil
		}
		_, _ = stdout.Write(data)
		_, _ = io.WriteString(stdout, "\n")
	} else {
		// last: applies to the text output only; --json always carries every
		// annotation so scripts get a stable, complete document.
		_, _ = io.WriteString(stdout, FormatTaskInfoLast(tasks[0], displayTaskAlias(tasks[0].UUID, aliases), aliases, last))
	}
	return 0, nil
}

func (d *Dispatcher) infoTasks(ctx context.Context, args []string, stderr io.Writer) ([]TaskExport, int, error) {
	if len(args) >= 2 {
		_, tasks, code, err := d.resolveTaskSelector(ctx, args[1], stderr)
		return tasks, code, err
	}
	return d.startedInfoTasks(ctx, stderr)
}

func (d *Dispatcher) startedInfoTasks(ctx context.Context, stderr io.Writer) ([]TaskExport, int, error) {
	tasks, code, err := d.exportTasks(ctx, []string{startedFilter, "export"}, stderr)
	if err != nil {
		return nil, code, err
	}
	switch len(tasks) {
	case 0:
		return nil, 1, fmt.Errorf("no started task found")
	case 1:
		return tasks, 0, nil
	default:
		return nil, 1, fmt.Errorf("multiple started tasks found; pass an ID or UUID explicitly")
	}
}

func (d *Dispatcher) exportTasks(ctx context.Context, args []string, stderr io.Writer) ([]TaskExport, int, error) {
	var outBuf bytes.Buffer
	code, err := d.runner.Run(ctx, args, nil, &outBuf, stderr)
	if code != 0 {
		return nil, code, err
	}
	tasks, err := ParseTaskExport(&outBuf)
	if err != nil {
		return nil, 1, err
	}
	if len(tasks) == 0 && len(args) > 0 && strings.HasPrefix(args[0], "uuid:") {
		return nil, 1, fmt.Errorf("task not found")
	}
	return tasks, 0, nil
}

func writeInfoError(stderr io.Writer, err error) {
	msg := err.Error()
	if strings.HasPrefix(msg, "error:") {
		_, _ = io.WriteString(stderr, msg+"\n")
		return
	}
	_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
}
