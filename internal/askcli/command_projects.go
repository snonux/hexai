package askcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type taskCommandRunner func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error

func (d *Dispatcher) handleProjects(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	cmdArgs, err := d.projectsExportArgs(ctx, args[1:])
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1, nil
	}
	taskPath, err := d.projectTaskBinary()()
	if err != nil {
		return 1, fmt.Errorf("ask projects: task binary lookup failed: %w", err)
	}
	var outBuf bytes.Buffer
	err = d.projectTaskCommand()(ctx, taskPath, cmdArgs, nil, &outBuf, stderr)
	if err != nil {
		return exitCodeFor(err), fmt.Errorf("ask projects: task export failed: %w", err)
	}
	tasks, err := ParseTaskExport(&outBuf)
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to parse task data: %v\n", err)
		return 1, nil
	}
	return d.writeProjects(unstartedProjects(tasks), stdout, stderr)
}

// projectsExportArgs builds the export command for ask projects from the
// user-supplied filters: +tags and the due-within:/due-window:/since:
// shortcuts. Other args are ignored.
func (d *Dispatcher) projectsExportArgs(ctx context.Context, filters []string) ([]string, error) {
	scopeFilter := taskScopeFilter(taskScopeFromContext(ctx))
	// Filters must come before `export`; taskwarrior treats trailing args as report names.
	cmdArgs := []string{"rc.verbose=nothing", "rc.confirmation=off", scopeFilter, "status:pending"}
	for _, arg := range filters {
		if filter, ok, err := resolveDateShortcut(arg, d.now()); ok {
			if err != nil {
				return nil, err
			}
			cmdArgs = append(cmdArgs, filter)
			continue
		}
		if strings.HasPrefix(arg, "+") {
			cmdArgs = append(cmdArgs, arg)
		}
	}
	return append(cmdArgs, "export"), nil
}

// unstartedProjects returns the sorted, unique projects of tasks that are
// pending and not yet started.
func unstartedProjects(tasks []TaskExport) []string {
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
	return projects
}

// writeProjects prints projects one per line, or as a JSON array.
func (d *Dispatcher) writeProjects(projects []string, stdout, stderr io.Writer) (int, error) {
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

func (d *Dispatcher) projectTaskBinary() func() (string, error) {
	if d != nil && d.findTaskBinary != nil {
		return d.findTaskBinary
	}
	return findTaskBinary
}

func (d *Dispatcher) projectTaskCommand() taskCommandRunner {
	if d != nil && d.runTaskCommand != nil {
		return d.runTaskCommand
	}
	return runTaskCommand
}
