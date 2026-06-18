package askcli

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// verifyDependenciesCompletedForStart returns 0 if every dependency is completed,
// or 1 after writing an error to stderr when the task must not be started yet.
func (d *Dispatcher) verifyDependenciesCompletedForStart(ctx context.Context, task TaskExport, stderr io.Writer) int {
	if len(task.Depends) == 0 {
		return 0
	}
	aliases, err := d.aliasCache.withDefaults().ensureTaskAliasesForUUIDs(task.Depends)
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to load task aliases: %v\n", err)
		return 1
	}
	var incomplete []string
	for _, depUUID := range task.Depends {
		depTasks, code, err := d.exportTasks(ctx, []string{"uuid:" + depUUID, "export"}, stderr)
		if err != nil {
			writeInfoError(stderr, err)
			return code
		}
		if len(depTasks) == 0 {
			fmt.Fprintf(stderr, "error: dependency task not found (%s)\n", displayTaskAlias(depUUID, aliases))
			return 1
		}
		status := strings.ToLower(strings.TrimSpace(depTasks[0].Status))
		if status != "completed" {
			label := displayTaskAlias(depUUID, aliases)
			incomplete = append(incomplete, fmt.Sprintf("%s (%s)", label, depTasks[0].Status))
		}
	}
	if len(incomplete) > 0 {
		fmt.Fprintf(stderr, "error: cannot start until all dependencies are completed; incomplete: %s\n",
			strings.Join(incomplete, ", "))
		return 1
	}
	return 0
}
