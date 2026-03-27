package askcli

import "context"

type taskScopeMode int

const (
	taskScopeAgent taskScopeMode = iota
	taskScopeNoAgent
)

type taskScopeContextKey struct{}

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

func taskScopeFilter(scope taskScopeMode) string {
	if scope == taskScopeNoAgent {
		return "-agent"
	}
	return "+agent"
}

func parseTaskScopePrefix(args []string) (taskScopeMode, []string) {
	if len(args) == 0 {
		return taskScopeAgent, nil
	}
	if isTaskScopePrefix(args[0]) {
		return taskScopeNoAgent, args[1:]
	}
	return taskScopeAgent, args
}

func isTaskScopePrefix(arg string) bool {
	switch arg {
	case "na", "no-agent":
		return true
	default:
		return false
	}
}

func trimTaskScopePrefix(args []string) []string {
	if len(args) == 0 || !isTaskScopePrefix(args[0]) {
		return args
	}
	return args[1:]
}
