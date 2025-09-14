package hexaiaction

import (
    "context"
    "strings"
    "testing"

    "codeberg.org/snonux/hexai/internal/appconfig"
    "codeberg.org/snonux/hexai/internal/llm"
)

// capDoer captures last LLM messages for assertions.
type capDoer struct{ last []llm.Message }
func (c *capDoer) Chat(_ context.Context, msgs []llm.Message, _ ...llm.RequestOption) (string, error) { c.last = append([]llm.Message{}, msgs...); return "OK", nil }
func (*capDoer) DefaultModel() string { return "m" }

func TestExecuteAction_Custom_ClearsSelection(t *testing.T) {
    cfg := appconfig.Load(nil)
    parts := InputParts{Selection: "code"}
    selectedCustom = &appconfig.CustomAction{ID: "x", Title: "X", Instruction: "Do it"}
    _, _ = executeAction(context.Background(), ActionCustom, parts, cfg, fakeDoer{"OK"}, nil)
    if selectedCustom != nil {
        t.Fatalf("expected selectedCustom cleared after execution")
    }
}

func TestRunCustom_UserTemplate_InjectsDiagnostics(t *testing.T) {
    cfg := appconfig.Load(nil)
    parts := InputParts{Selection: "code", Diagnostics: []string{"L1", "L2"}}
    ca := appconfig.CustomAction{ID: "y", Title: "Y", User: "{{diagnostics}}\n{{selection}}"}
    cap := &capDoer{}
    _, err := runCustom(context.Background(), cfg, cap, ca, parts)
    if err != nil { t.Fatalf("runCustom error: %v", err) }
    if len(cap.last) == 0 {
        t.Fatalf("expected messages captured")
    }
    // user message should contain diagnostics and selection
    found := false
    for _, m := range cap.last {
        if m.Role == "user" && strings.Contains(m.Content, "L1") && strings.Contains(m.Content, "code") {
            found = true
        }
    }
    if !found {
        t.Fatalf("expected diagnostics and selection in user message: %+v", cap.last)
    }
}

