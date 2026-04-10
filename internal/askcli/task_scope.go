package askcli

import (
	"context"
	"strings"
)

type taskScopeMode int

const (
	taskScopeAgent taskScopeMode = iota
	taskScopeNoAgent
)

type taskScopeContextKey struct{}

type taskProjectContextKey struct{}

type taskProjectContextValue struct {
	project string
}

func contextWithTaskScope(ctx context.Context, scope taskScopeMode) context.Context {
	if scope == taskScopeAgent {
		return ctx
	}
	return context.WithValue(ctx, taskScopeContextKey{}, scope)
}

func taskScopeFromContext(ctx context.Context) taskScopeMode {
	if ctx == nil {
		return taskScopeAgent
	}
	scope, ok := ctx.Value(taskScopeContextKey{}).(taskScopeMode)
	if !ok {
		return taskScopeAgent
	}
	return scope
}

func contextWithTaskProject(ctx context.Context, project string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, taskProjectContextKey{}, taskProjectContextValue{project: project})
}

func taskProjectFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	value, ok := ctx.Value(taskProjectContextKey{}).(taskProjectContextValue)
	if !ok {
		return "", false
	}
	return value.project, true
}

func taskScopeFilter(scope taskScopeMode) string {
	if scope == taskScopeNoAgent {
		return "-agent"
	}
	return "+agent"
}

func parseTaskScopePrefix(args []string) (taskScopeMode, []string) {
	scope, _, _, remaining := parseTaskPrefixes(args)
	return scope, remaining
}

func isTaskScopePrefix(arg string) bool {
	switch arg {
	case "na", "no-agent":
		return true
	default:
		return false
	}
}

func isTaskProjectPrefix(arg string) bool {
	return strings.HasPrefix(arg, "proj:")
}

func trimTaskScopePrefix(args []string) []string {
	return trimTaskPrefixes(args)
}

func trimTaskPrefixes(args []string) []string {
	_, _, _, remaining := parseTaskPrefixes(args)
	return remaining
}

func parseTaskPrefixes(args []string) (taskScopeMode, string, bool, []string) {
	scope := taskScopeAgent
	projectName := ""
	projectSet := false
	for len(args) > 0 {
		switch {
		case isTaskScopePrefix(args[0]):
			scope = taskScopeNoAgent
			args = args[1:]
		case isTaskProjectPrefix(args[0]):
			projectName = args[0][len("proj:"):]
			projectSet = true
			args = args[1:]
		default:
			return scope, projectName, projectSet, args
		}
	}
	return scope, projectName, projectSet, nil
}
