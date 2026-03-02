package hexaiaction

import (
	"bytes"
	"context"
	"os"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/editor"
	"codeberg.org/snonux/hexai/internal/llm"
)

type llmFake2 struct{}

func (llmFake2) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return "DONE", nil
}
func (llmFake2) Name() string         { return "fake" }
func (llmFake2) DefaultModel() string { return "m" }

func TestActionCustom_UsesEditorPrompt(t *testing.T) {
	// Isolate from user config that might enable custom menu/TUI.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Seam: choose custom, fake client, and fake editor.
	runner := NewRunner()
	runner.chooseAction = func(_ appconfig.App) (actionChoice, error) {
		return actionChoice{kind: ActionCustomPrompt}, nil
	}
	runner.newClient = func(_ appconfig.App) (actionClient, error) { return llmFake2{}, nil }

	oldRunEd := editor.RunEditor
	editor.RunEditor = func(_ string, path string) error {
		return os.WriteFile(path, []byte("make it done"), 0o600)
	}
	t.Cleanup(func() { editor.RunEditor = oldRunEd })
	t.Setenv("HEXAI_EDITOR", "dummy")

	in := bytes.NewBufferString("some code")
	var out bytes.Buffer
	var errb bytes.Buffer
	if err := runner.Run(context.Background(), in, &out, &errb); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.String() == "" {
		t.Fatalf("expected output")
	}
}
