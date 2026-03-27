package askcli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

func (d *Dispatcher) handleAdd(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) < 2 {
		_, _ = io.WriteString(stderr, "error: ask add requires a description\n")
		return 1, nil
	}
	modifiers, description, dependencySelectors, err := parseAddArgs(args[1:])
	if err != nil {
		writeInfoError(stderr, err)
		return 1, nil
	}
	if strings.TrimSpace(description) == "" {
		_, _ = io.WriteString(stderr, "error: ask add requires a description\n")
		return 1, nil
	}
	dependencyUUIDs, code, err := d.resolveAddDependencyUUIDs(ctx, dependencySelectors, stderr)
	if err != nil {
		writeInfoError(stderr, err)
		return code, nil
	}
	var outBuf bytes.Buffer
	// rc.verbose=nothing keeps Taskwarrior quiet by default. rc.verbose=new-uuid
	// then re-enables the UUID-only confirmation we parse below.
	taskArgs := []string{"add", "rc.verbose=nothing", "rc.verbose=new-uuid"}
	taskArgs = append(taskArgs, modifiers...)
	if len(dependencyUUIDs) > 0 {
		taskArgs = append(taskArgs, "depends:"+strings.Join(dependencyUUIDs, ","))
	}
	taskArgs = append(taskArgs, description)
	code, err = d.runner.Run(ctx, taskArgs, nil, &outBuf, stderr)
	if code != 0 {
		return code, err
	}
	uuid := extractUUIDFromAddOutput(outBuf.String())
	if uuid == "" {
		_, _ = io.WriteString(stderr, "error: could not parse UUID from task creation output\n")
		return 1, nil
	}
	aliases, err := ensureTaskAliasesForUUIDs([]string{uuid})
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to assign task alias: %v\n", err)
		return 1, nil
	}
	_, _ = io.WriteString(stdout, displayTaskAlias(uuid, aliases)+"\n")
	return 0, nil
}

func (d *Dispatcher) resolveAddDependencyUUIDs(ctx context.Context, selectors []string, stderr io.Writer) ([]string, int, error) {
	dependencies := make([]string, 0, len(selectors))
	for _, selector := range selectors {
		resolved, _, code, err := d.resolveTaskSelector(ctx, selector, stderr)
		if err != nil {
			return nil, code, err
		}
		dependencies = append(dependencies, resolved.UUID)
	}
	return dependencies, 0, nil
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

// parseAddArgs splits args into taskwarrior modifier tokens, the description,
// and optional dependency selectors introduced by depends:<id>[,<id>...]
// modifier tokens.
// Modifier tokens are args that start with "priority:", "+", or "-" AND contain
// no spaces (tags and priority flags cannot have spaces). The first arg that is
// not a modifier begins the description; all remaining args are joined with
// spaces. Dependency selectors must be provided before the description via one
// or more depends: selectors.
func parseAddArgs(args []string) (modifiers []string, description string, dependencySelectors []string, err error) {
	for i, arg := range args {
		if strings.HasPrefix(arg, "depends:") {
			selectors, err := parseAddDependencySelectors(arg)
			if err != nil {
				return nil, "", nil, err
			}
			dependencySelectors = append(dependencySelectors, selectors...)
			continue
		}
		if isAddModifier(arg) {
			modifiers = append(modifiers, arg)
		} else {
			description = strings.Join(args[i:], " ")
			return
		}
	}
	// All args were modifiers; no description provided.
	return
}

func parseAddDependencySelectors(arg string) ([]string, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(arg, "depends:"))
	if raw == "" {
		return nil, fmt.Errorf("ask add depends:<id|uuid>[,<id|uuid>...] requires at least one dependency ID or UUID")
	}
	parts := strings.Split(raw, ",")
	selectors := make([]string, 0, len(parts))
	for _, part := range parts {
		selector := strings.TrimSpace(part)
		if selector == "" {
			return nil, fmt.Errorf("ask add dependency selector list contains an empty item")
		}
		selectors = append(selectors, selector)
	}
	return selectors, nil
}

func isAddModifier(arg string) bool {
	return !strings.Contains(arg, " ") &&
		(strings.HasPrefix(arg, "priority:") || strings.HasPrefix(arg, "+") || strings.HasPrefix(arg, "-"))
}
