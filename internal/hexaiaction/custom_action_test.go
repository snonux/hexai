package hexaiaction

import (
    "bytes"
    "context"
    "testing"

    "codeberg.org/snonux/hexai/internal/appconfig"
    "codeberg.org/snonux/hexai/internal/editor"
    "codeberg.org/snonux/hexai/internal/llm"
    "os"
)

type llmFake2 struct{}
func (llmFake2) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) { return "DONE", nil }
func (llmFake2) Name() string { return "fake" }
func (llmFake2) DefaultModel() string { return "m" }

func TestActionCustom_UsesEditorPrompt(t *testing.T) {
    // Seam: choose custom, fake client, and fake editor
    oldChoose := chooseActionFn
    oldNew := newClientFromApp
    chooseActionFn = func() (ActionKind, error) { return ActionCustom, nil }
    newClientFromApp = func(_ appconfig.App) (llm.Client, error) { return llmFake2{}, nil }
    t.Cleanup(func(){ chooseActionFn = oldChoose; newClientFromApp = oldNew })

    oldRunEd := editor.RunEditor
    editor.RunEditor = func(_ string, path string) error {
        return os.WriteFile(path, []byte("make it done"), 0o600)
    }
    t.Cleanup(func(){ editor.RunEditor = oldRunEd })
    t.Setenv("HEXAI_EDITOR", "dummy")

    in := bytes.NewBufferString("some code")
    var out bytes.Buffer
    var errb bytes.Buffer
    if err := Run(context.Background(), in, &out, &errb); err != nil { t.Fatalf("Run: %v", err) }
    if out.String() == "" { t.Fatalf("expected output") }
}
