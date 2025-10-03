package lsp

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"testing"
)

func TestHandleInitialize_Capabilities(t *testing.T) {
	var out bytes.Buffer
	s := newTestServer()
	s.logger = log.New(io.Discard, "", 0)
	s.out = &out
	cfg := s.cfg
	cfg.TriggerCharacters = []string{".", ":"}
	s.cfg = cfg
	req := Request{JSONRPC: "2.0", ID: json.RawMessage("7"), Method: "initialize"}
	out.Reset()
	s.handleInitialize(req)
	resp := captureResponse(t, &out)
	var init InitializeResult
	b, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(b, &init); err != nil {
		t.Fatalf("decode init: %v", err)
	}
	if init.Capabilities.CodeActionProvider == nil {
		t.Fatalf("expected codeActionProvider")
	}
	// CodeActionProvider is any; re-marshal to struct
	var cap struct {
		ResolveProvider bool `json:"resolveProvider"`
	}
	cb, _ := json.Marshal(init.Capabilities.CodeActionProvider)
	_ = json.Unmarshal(cb, &cap)
	if !cap.ResolveProvider {
		t.Fatalf("expected resolveProvider=true")
	}
	if init.Capabilities.CompletionProvider == nil || len(init.Capabilities.CompletionProvider.TriggerCharacters) == 0 {
		t.Fatalf("expected trigger characters")
	}
}

func TestIsTriggerEvent_Variants(t *testing.T) {
	s := newTestServer()
	cfg := s.cfg
	cfg.TriggerCharacters = []string{".", ":"}
	s.cfg = cfg
	// 1) Manual invoke via context
	ctx := struct {
		TriggerKind int `json:"triggerKind"`
	}{TriggerKind: 1}
	raw, _ := json.Marshal(ctx)
	p := CompletionParams{Position: Position{Line: 0, Character: 1}, Context: json.RawMessage(raw)}
	if !s.isTriggerEvent(p, "a") {
		t.Fatalf("manual invoke should trigger")
	}
	// 2) TriggerCharacter present and allowed
	ctx2 := struct {
		TriggerKind      int    `json:"triggerKind"`
		TriggerCharacter string `json:"triggerCharacter"`
	}{TriggerKind: 2, TriggerCharacter: "."}
	raw2, _ := json.Marshal(ctx2)
	p2 := CompletionParams{Position: Position{Line: 0, Character: 1}, Context: json.RawMessage(raw2)}
	if !s.isTriggerEvent(p2, "a.") {
		t.Fatalf("trigger char should trigger")
	}
	// 3) Fallback char left of cursor
	p3 := CompletionParams{Position: Position{Line: 0, Character: 3}}
	if !s.isTriggerEvent(p3, "ab:") {
		t.Fatalf("fallback char should trigger")
	}
	// 4) Bare double-open disables trigger
	p4 := CompletionParams{Position: Position{Line: 0, Character: 3}}
	if s.isTriggerEvent(p4, ">>!") {
		t.Fatalf("bare double-open should not trigger")
	}
}
