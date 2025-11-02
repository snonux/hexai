package lsp

import (
	"encoding/json"
	"testing"
)

func TestResolveCodeAction_UsesRewritePrompts(t *testing.T) {
	s := newTestServer()
	cap := &captureLLM{}
	s.llmClient = cap
	cfg := s.cfg
	cfg.PromptCodeActionRewriteSystem = "RSYS"
	cfg.PromptCodeActionRewriteUser = "RUSER {{instruction}} {{selection}}"
	s.cfg = cfg
	uri := "file:///x.go"
	s.setDocument(uri, "package p\nvar a=1\n")
	payload := struct {
		Type        string `json:"type"`
		URI         string `json:"uri"`
		Range       Range  `json:"range"`
		Instruction string `json:"instruction"`
		Selection   string `json:"selection"`
	}{Type: "rewrite", URI: uri, Range: Range{Start: Position{Line: 1}, End: Position{Line: 1, Character: 5}}, Instruction: "do it", Selection: "var a"}
	raw, _ := json.Marshal(payload)
	ca := CodeAction{Title: "Hexai: rewrite selection", Data: raw}
	_, _ = s.resolveCodeAction(ca)
	if len(cap.msgs) < 2 {
		t.Fatalf("expected chat messages")
	}
	if cap.msgs[0].Content != "RSYS" || cap.msgs[1].Role != "user" || cap.msgs[1].Content != "RUSER do it var a" {
		t.Fatalf("unexpected rewrite prompts: %#v", cap.msgs)
	}
}

func TestResolveCodeAction_UsesDiagnosticsPrompts(t *testing.T) {
	s := newTestServer()
	cap := &captureLLM{}
	s.llmClient = cap
	cfg := s.cfg
	cfg.PromptCodeActionDiagnosticsSystem = "DSYS"
	cfg.PromptCodeActionDiagnosticsUser = "DUSER {{diagnostics}} {{selection}}"
	s.cfg = cfg
	uri := "file:///x.go"
	s.setDocument(uri, "package p\nvar a=1\n")
	payload := struct {
		Type        string       `json:"type"`
		URI         string       `json:"uri"`
		Range       Range        `json:"range"`
		Selection   string       `json:"selection"`
		Diagnostics []Diagnostic `json:"diagnostics"`
	}{Type: "diagnostics", URI: uri, Range: Range{Start: Position{Line: 1}}, Selection: "var a", Diagnostics: []Diagnostic{{Message: "oops1"}, {Message: "oops2"}}}
	raw, _ := json.Marshal(payload)
	ca := CodeAction{Title: "Hexai: resolve diagnostics", Data: raw}
	_, _ = s.resolveCodeAction(ca)
	if len(cap.msgs) < 2 {
		t.Fatalf("expected chat messages")
	}
	if cap.msgs[0].Content != "DSYS" || cap.msgs[1].Role != "user" {
		t.Fatalf("unexpected diagnostics prompts: %#v", cap.msgs)
	}
	if got := cap.msgs[1].Content; !contains(got, "oops1") || !contains(got, "oops2") || !contains(got, "var a") {
		t.Fatalf("diagnostics/user content mismatch: %q", got)
	}
}

func TestResolveCodeAction_UsesDocumentPrompts(t *testing.T) {
	s := newTestServer()
	cap := &captureLLM{}
	s.llmClient = cap
	cfg := s.cfg
	cfg.PromptCodeActionDocumentSystem = "DOCSYS"
	cfg.PromptCodeActionDocumentUser = "DOCUSER {{selection}}"
	s.cfg = cfg
	uri := "file:///x.go"
	s.setDocument(uri, "package p\nvar a=1\n")
	payload := struct {
		Type      string `json:"type"`
		URI       string `json:"uri"`
		Range     Range  `json:"range"`
		Selection string `json:"selection"`
	}{Type: "document", URI: uri, Range: Range{Start: Position{Line: 1}}, Selection: "var a"}
	raw, _ := json.Marshal(payload)
	ca := CodeAction{Title: "Hexai: document selection", Data: raw}
	_, _ = s.resolveCodeAction(ca)
	if len(cap.msgs) < 2 {
		t.Fatalf("expected chat messages")
	}
	if cap.msgs[0].Content != "DOCSYS" || cap.msgs[1].Content != "DOCUSER var a" {
		t.Fatalf("unexpected document prompts: %#v", cap.msgs)
	}
}

func TestGenerateGoTest_UsesPrompts(t *testing.T) {
	s := newTestServer()
	cap := &captureLLM{}
	s.llmClient = cap
	cfg := s.cfg
	cfg.PromptCodeActionGoTestSystem = "GTSYS"
	cfg.PromptCodeActionGoTestUser = "GTUSER {{function}}"
	s.cfg = cfg
	_ = s.generateGoTestFunction("func Add(a,b int) int {return a+b}")
	if len(cap.msgs) < 2 {
		t.Fatalf("expected chat messages")
	}
	if cap.msgs[0].Content != "GTSYS" || !contains(cap.msgs[1].Content, "func Add") {
		t.Fatalf("unexpected gotest prompts: %#v", cap.msgs)
	}
}
