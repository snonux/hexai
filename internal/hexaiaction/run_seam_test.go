package hexaiaction

import (
    "bytes"
    "context"
    "testing"

    "codeberg.org/snonux/hexai/internal/appconfig"
    "codeberg.org/snonux/hexai/internal/llm"
)

type llmFake struct{}

func (llmFake) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) { return "OK", nil }
func (llmFake) Name() string         { return "fake" }
func (llmFake) DefaultModel() string { return "model" }

func TestRun_WithSeams_SkipAndRewrite(t *testing.T) {
    // Seam: choose action to Skip first, then Rewrite
    oldChoose := chooseActionFn
    oldNew := newClientFromApp
    t.Cleanup(func(){ chooseActionFn = oldChoose; newClientFromApp = oldNew })
    // 1) Skip -> echoes selection
    chooseActionFn = func() (ActionKind, error) { return ActionSkip, nil }
    newClientFromApp = func(_ appconfig.App) (llm.Client, error) { return llmFake{}, nil }
    var out bytes.Buffer
    in := bytes.NewBufferString("some code")
    if err := Run(context.Background(), in, &out, &out); err != nil { t.Fatalf("Run skip: %v", err) }
    if out.String() != "some code" { t.Fatalf("skip out: %q", out.String()) }
    // 2) Rewrite -> requires inline instruction
    chooseActionFn = func() (ActionKind, error) { return ActionRewrite, nil }
    out.Reset()
    in = bytes.NewBufferString(";upper;\nhello")
    if err := Run(context.Background(), in, &out, &out); err != nil { t.Fatalf("Run rewrite: %v", err) }
    if out.String() == "" { t.Fatalf("expected non-empty rewrite output") }
}
