package hexaiaction

import (
	"context"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

func TestActionHandlers_AreSelfRegistered(t *testing.T) {
	handlers := codeActionHandlers()
	for _, kind := range []ActionKind{
		ActionSkip,
		ActionRewrite,
		ActionDiagnostics,
		ActionDocument,
		ActionGoTest,
		ActionSimplify,
		ActionFixTypos,
		ActionCustom,
		ActionCustomPrompt,
	} {
		if _, ok := handlers[kind]; !ok {
			t.Fatalf("expected handler for %q", kind)
		}
	}
}

func TestActionHandlers_SnapshotDoesNotMutateRegistry(t *testing.T) {
	handlers := codeActionHandlers()
	delete(handlers, ActionSkip)
	if _, ok := lookupActionHandler(ActionSkip); !ok {
		t.Fatal("mutating handler snapshot changed registry")
	}
}

func TestExecuteAction_UnknownFallsBackToSelection(t *testing.T) {
	cfg := appconfig.App{}
	parts := InputParts{Selection: "original"}
	out, err := executeAction(context.Background(), ActionKind("missing"), parts, &cfg, fakeDoer{"ignored"}, nil, nil)
	if err != nil {
		t.Fatalf("executeAction: %v", err)
	}
	if out != "original" {
		t.Fatalf("expected fallback selection, got %q", out)
	}
}

func TestActionHandlerRegistryRejectsInvalidRegistrations(t *testing.T) {
	tests := map[string]func(*actionHandlerRegistry){
		"empty kind": func(registry *actionHandlerRegistry) {
			registry.register("", actionHandlerFunc(handleSkipAction))
		},
		"nil handler": func(registry *actionHandlerRegistry) {
			registry.register(ActionKind("nil"), nil)
		},
		"duplicate kind": func(registry *actionHandlerRegistry) {
			registry.register(ActionKind("dup"), actionHandlerFunc(handleSkipAction))
			registry.register(ActionKind("dup"), actionHandlerFunc(handleSkipAction))
		},
	}

	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered == nil {
					t.Fatal("expected panic")
				}
			}()
			run(newActionHandlerRegistry())
		})
	}
}

func TestActionHandlerRegistryNilPanicNamesKind(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic")
		}
		if !strings.Contains(recovered.(string), "custom") {
			t.Fatalf("expected panic to name kind, got %v", recovered)
		}
	}()
	newActionHandlerRegistry().register(ActionCustom, nil)
}
