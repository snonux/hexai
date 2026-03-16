package hexaiaction

import (
	"context"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
)

type fakeDoer struct{ out string }

func (f fakeDoer) Chat(_ context.Context, _ []llm.Message, _ ...llm.RequestOption) (string, error) {
	return f.out, nil
}
func (f fakeDoer) DefaultModel() string { return "m" }

func TestExecuteAction_Skip(t *testing.T) {
	cfg := appconfig.App{}
	parts := InputParts{Selection: "data"}
	out, err := executeAction(context.Background(), ActionSkip, parts, &cfg, fakeDoer{"IGN"}, nil, nil)
	if err != nil || out != "data" {
		t.Fatalf("skip failed: %q %v", out, err)
	}
}

func TestExecuteAction_Rewrite_Document_GoTest(t *testing.T) {
	cfg := appconfig.Load(nil) // defaults
	// Use fenced output to exercise StripFences
	client := fakeDoer{"```\nDONE\n```"}

	// rewrite with inline instruction
	sel := ";change;\ncode"
	out, err := executeAction(context.Background(), ActionRewrite, InputParts{Selection: sel}, &cfg, client, nil, nil)
	if err != nil || strings.TrimSpace(out) != "DONE" {
		t.Fatalf("rewrite failed: %q %v", out, err)
	}

	// document
	out, err = executeAction(context.Background(), ActionDocument, InputParts{Selection: "code"}, &cfg, client, nil, nil)
	if err != nil || strings.TrimSpace(out) != "DONE" {
		t.Fatalf("document failed: %q %v", out, err)
	}

	// go test
	out, err = executeAction(context.Background(), ActionGoTest, InputParts{Selection: "func A(){}"}, &cfg, client, nil, nil)
	if err != nil || strings.TrimSpace(out) != "DONE" {
		t.Fatalf("gotest failed: %q %v", out, err)
	}
}
