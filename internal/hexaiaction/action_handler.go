package hexaiaction

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"sync"

	"github.com/snonux/hexai/internal/appconfig"
)

type actionRequest struct {
	parts          InputParts
	cfg            actionConfig
	client         chatDoer
	stderr         io.Writer
	selectedCustom *appconfig.CustomAction
}

// ActionHandler executes one tmux action kind.
type ActionHandler interface {
	Execute(context.Context, actionRequest) (string, error)
}

// CodeActionHandler is kept as a compatibility name for tmux code-action handlers.
type CodeActionHandler = ActionHandler

type actionHandlerFunc func(context.Context, actionRequest) (string, error)

func (f actionHandlerFunc) Execute(ctx context.Context, req actionRequest) (string, error) {
	if f == nil {
		return "", fmt.Errorf("hexaiaction: nil action handler")
	}
	return f(ctx, req)
}

type actionHandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[ActionKind]ActionHandler
}

func newActionHandlerRegistry() *actionHandlerRegistry {
	return &actionHandlerRegistry{handlers: make(map[ActionKind]ActionHandler)}
}

func (r *actionHandlerRegistry) register(kind ActionKind, handler ActionHandler) {
	if kind == "" {
		panic("hexaiaction: cannot register empty action kind")
	}
	if isNilActionHandler(handler) {
		panic(fmt.Sprintf("hexaiaction: cannot register nil handler for %q", kind))
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[kind]; exists {
		panic(fmt.Sprintf("hexaiaction: handler already registered for %q", kind))
	}
	r.handlers[kind] = handler
}

func isNilActionHandler(handler ActionHandler) bool {
	if handler == nil {
		return true
	}

	value := reflect.ValueOf(handler)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (r *actionHandlerRegistry) lookup(kind ActionKind) (ActionHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[kind]
	return handler, ok
}

func (r *actionHandlerRegistry) snapshot() map[ActionKind]ActionHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handlers := make(map[ActionKind]ActionHandler, len(r.handlers))
	for kind, handler := range r.handlers {
		handlers[kind] = handler
	}
	return handlers
}

var actionHandlers = newActionHandlerRegistry()

func registerActionHandler(kind ActionKind, handler ActionHandler) {
	actionHandlers.register(kind, handler)
}

func lookupActionHandler(kind ActionKind) (ActionHandler, bool) {
	return actionHandlers.lookup(kind)
}

func codeActionHandlers() map[ActionKind]CodeActionHandler {
	return actionHandlers.snapshot()
}
