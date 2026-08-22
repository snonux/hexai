package hexaiaction

import (
	"context"
	"strings"
	"testing"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/llm"
)

func TestParseInput_NoDiagnostics(t *testing.T) {
	in := "some code here"
	parts, err := ParseInput(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if parts.Selection != in || len(parts.Diagnostics) != 0 {
		t.Fatalf("unexpected parse: %#v", parts)
	}
}

func TestParseInput_WithDiagnostics(t *testing.T) {
	in := "Diagnostics:\nmissing return\nuse of undefined: foo\n\nfunc a() {}"
	parts, err := ParseInput(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if parts.Selection != "func a() {}" {
		t.Fatalf("selection wrong: %q", parts.Selection)
	}
	if len(parts.Diagnostics) != 2 || parts.Diagnostics[0] != "missing return" {
		t.Fatalf("diags wrong: %#v", parts.Diagnostics)
	}
}

func TestExtractInstruction_Variants(t *testing.T) {
	cases := []struct{ in, wantInstr string }{
		{";rewrite to X;\ncode", "rewrite to X"},
		{"/* fix it */\ncode", "fix it"},
		{"<!-- doc me -->\ncode", "doc me"},
		{"// change it\ncode", "change it"},
		{"# tweak\ncode", "tweak"},
		{"-- fix\ncode", "fix"},
	}
	for _, c := range cases {
		got, cleaned := ExtractInstruction(c.in)
		if got != c.wantInstr {
			t.Fatalf("instr mismatch: %q != %q", got, c.wantInstr)
		}
		if strings.Contains(cleaned, c.wantInstr) && strings.Contains(c.in, c.wantInstr) {
			t.Fatalf("expected instruction removed from selection: %q", cleaned)
		}
	}
}

func TestRenderAndStrip(t *testing.T) {
	tpl := "Hello, {{name}}"
	out := Render(tpl, map[string]string{"name": "Hex"})
	if out != "Hello, Hex" {
		t.Fatalf("unexpected render: %q", out)
	}
	fenced := "```go\npackage x\n```"
	if StripFences(fenced) != "package x" {
		t.Fatalf("unexpected strip")
	}
}

type fakeClient struct {
	last []llm.Message
	out  string
	err  error
}

func (f *fakeClient) Chat(_ context.Context, msgs []llm.Message, _ ...llm.RequestOption) (string, error) {
	f.last = msgs
	return f.out, f.err
}

func (f *fakeClient) DefaultModel() string { return "m" }

func TestRuners_Prompts(t *testing.T) {
	cfg := appconfig.App{
		PromptConfig: appconfig.PromptConfig{
			PromptCodeActionRewriteSystem:     "SYS-R",
			PromptCodeActionRewriteUser:       "R {{instruction}} :: {{selection}}",
			PromptCodeActionDiagnosticsSystem: "SYS-D",
			PromptCodeActionDiagnosticsUser:   "D {{diagnostics}} :: {{selection}}",
			PromptCodeActionDocumentSystem:    "SYS-C",
			PromptCodeActionDocumentUser:      "C {{selection}}",
			PromptCodeActionGoTestSystem:      "SYS-T",
			PromptCodeActionGoTestUser:        "T {{function}}",
		},
	}
	f := &fakeClient{out: "```\nDONE\n```"}
	ctx := context.Background()
	// rewrite
	if out, err := runRewrite(ctx, &cfg, f, "instr", "sel"); err != nil || out != "DONE" {
		t.Fatalf("rewrite failed: %q %v", out, err)
	}
	if len(f.last) != 2 || f.last[0].Content != "SYS-R" || !strings.Contains(f.last[1].Content, "instr") {
		t.Fatalf("rewrite prompts wrong: %#v", f.last)
	}
	// diagnostics
	if out, err := runDiagnostics(ctx, &cfg, f, []string{"a", "b"}, "sel"); err != nil || out != "DONE" {
		t.Fatalf("diagnostics failed: %q %v", out, err)
	}
	if f.last[0].Content != "SYS-D" || !strings.Contains(f.last[1].Content, "a\nb") {
		t.Fatalf("diagnostics prompts wrong: %#v", f.last)
	}
	// document
	if out, err := runDocument(ctx, &cfg, f, "sel"); err != nil || out != "DONE" {
		t.Fatalf("document failed: %q %v", out, err)
	}
	if f.last[0].Content != "SYS-C" || !strings.Contains(f.last[1].Content, "sel") {
		t.Fatalf("document prompts wrong: %#v", f.last)
	}
	// gotest
	if out, err := runGoTest(ctx, &cfg, f, "func A(){}"); err != nil || out != "DONE" {
		t.Fatalf("gotest failed: %q %v", out, err)
	}
	if f.last[0].Content != "SYS-T" || !strings.Contains(f.last[1].Content, "func A(){") {
		t.Fatalf("gotest prompts wrong: %#v", f.last)
	}
}
