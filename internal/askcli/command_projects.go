package askcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

var (
	projectsFindTaskBinary = findTaskBinary
	projectsRunTaskCommand = runTaskCommand
)

func (d *Dispatcher) handleProjects(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	_ = args

	taskPath, err := projectsFindTaskBinary()
	if err != nil {
		return 1, fmt.Errorf("ask projects: task binary lookup failed: %w", err)
	}

	scopeFilter := taskScopeFilter(taskScopeFromContext(ctx))
	cmdArgs := append([]string{"rc.verbose=nothing", "rc.confirmation=off", scopeFilter, "status:pending", "export"}, args[1:]...)
	var outBuf bytes.Buffer
	err = projectsRunTaskCommand(ctx, taskPath, cmdArgs, nil, &outBuf, stderr)
	if err != nil {
		return exitCodeFor(err), fmt.Errorf("ask projects: task export failed: %w", err)
	}

	tasks, err := ParseTaskExport(&outBuf)
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to parse task data: %v\n", err)
		return 1, nil
	}

	projectSet := make(map[string]struct{})
	for _, task := range tasks {
		if task.Status == "pending" && task.Start == "" && task.Project != "" {
			projectSet[task.Project] = struct{}{}
		}
	}

	projects := make([]string, 0, len(projectSet))
	for p := range projectSet {
		projects = append(projects, p)
	}
	sort.Strings(projects)

	if d.jsonOutput {
		data, err := json.Marshal(projects)
		if err != nil {
			fmt.Fprintf(stderr, "error: failed to marshal JSON: %v\n", err)
			return 1, nil
		}
		_, _ = stdout.Write(data)
		_, _ = io.WriteString(stdout, "\n")
		return 0, nil
	}

	for _, p := range projects {
		_, _ = io.WriteString(stdout, p+"\n")
	}
	return 0, nil
}
