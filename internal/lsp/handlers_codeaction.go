// Summary: Code Action handlers and helpers split from handlers.go for clarity.
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"hexai/internal/llm"
	"hexai/internal/logging"
	"strings"
	"time"
)

func (s *Server) handleCodeAction(req Request) {
	var p CodeActionParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		if len(req.ID) != 0 {
			s.reply(req.ID, []CodeAction{}, nil)
		}
		return
	}
	d := s.getDocument(p.TextDocument.URI)
	if d == nil || len(d.lines) == 0 || s.llmClient == nil {
		if len(req.ID) != 0 {
			s.reply(req.ID, []CodeAction{}, nil)
		}
		return
	}
	sel := extractRangeText(d, p.Range)
	if strings.TrimSpace(sel) == "" {
		if len(req.ID) != 0 {
			s.reply(req.ID, []CodeAction{}, nil)
		}
		return
	}

	actions := make([]CodeAction, 0, 2)
	if a := s.buildRewriteCodeAction(p, sel); a != nil {
		actions = append(actions, *a)
	}
	if a := s.buildDiagnosticsCodeAction(p, sel); a != nil {
		actions = append(actions, *a)
	}
	if len(req.ID) != 0 {
		s.reply(req.ID, actions, nil)
	}
}

func (s *Server) buildRewriteCodeAction(p CodeActionParams, sel string) *CodeAction {
	if instr, cleaned := instructionFromSelection(sel); strings.TrimSpace(instr) != "" {
		payload := struct {
			Type        string `json:"type"`
			URI         string `json:"uri"`
			Range       Range  `json:"range"`
			Instruction string `json:"instruction"`
			Selection   string `json:"selection"`
		}{Type: "rewrite", URI: p.TextDocument.URI, Range: p.Range, Instruction: instr, Selection: cleaned}
		raw, _ := json.Marshal(payload)
		ca := CodeAction{Title: "Hexai: rewrite selection", Kind: "refactor.rewrite", Data: raw}
		return &ca
	}
	return nil
}

func (s *Server) buildDiagnosticsCodeAction(p CodeActionParams, sel string) *CodeAction {
	diags := s.diagnosticsInRange(p.Context, p.Range)
	if len(diags) == 0 {
		return nil
	}
	payload := struct {
		Type        string       `json:"type"`
		URI         string       `json:"uri"`
		Range       Range        `json:"range"`
		Selection   string       `json:"selection"`
		Diagnostics []Diagnostic `json:"diagnostics"`
	}{Type: "diagnostics", URI: p.TextDocument.URI, Range: p.Range, Selection: sel, Diagnostics: diags}
	raw, _ := json.Marshal(payload)
	ca := CodeAction{Title: "Hexai: resolve diagnostics", Kind: "quickfix", Data: raw}
	return &ca
}

func (s *Server) resolveCodeAction(ca CodeAction) (CodeAction, bool) {
	if s.llmClient == nil || len(ca.Data) == 0 {
		return ca, false
	}
	var payload struct {
		Type        string       `json:"type"`
		URI         string       `json:"uri"`
		Range       Range        `json:"range"`
		Instruction string       `json:"instruction,omitempty"`
		Selection   string       `json:"selection"`
		Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	}
	if err := json.Unmarshal(ca.Data, &payload); err != nil {
		return ca, false
	}
	switch payload.Type {
	case "rewrite":
		sys := "You are a precise code refactoring engine. Rewrite the given code strictly according to the instruction. Return only the updated code with no prose or backticks. Preserve formatting where reasonable."
		user := fmt.Sprintf("Instruction: %s\n\nSelected code to transform:\n%s", payload.Instruction, payload.Selection)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		messages := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
		opts := s.llmRequestOpts()
		if text, err := s.llmClient.Chat(ctx, messages, opts...); err == nil {
			if out := stripCodeFences(strings.TrimSpace(text)); out != "" {
				edit := WorkspaceEdit{Changes: map[string][]TextEdit{payload.URI: {{Range: payload.Range, NewText: out}}}}
				ca.Edit = &edit
				return ca, true
			}
		} else {
			logging.Logf("lsp ", "codeAction rewrite llm error: %v", err)
		}
	case "diagnostics":
		sys := "You are a precise code fixer. Resolve the given diagnostics by editing only the selected code. Return only the corrected code with no prose or backticks. Keep behavior and style, and avoid unrelated changes."
		var b strings.Builder
		b.WriteString("Diagnostics to resolve (selection only):\n")
		for i, dgn := range payload.Diagnostics {
			if dgn.Source != "" {
				fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, dgn.Source, dgn.Message)
			} else {
				fmt.Fprintf(&b, "%d. %s\n", i+1, dgn.Message)
			}
		}
		b.WriteString("\nSelected code:\n")
		b.WriteString(payload.Selection)
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		messages := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: b.String()}}
		opts := s.llmRequestOpts()
		if text, err := s.llmClient.Chat(ctx, messages, opts...); err == nil {
			if out := stripCodeFences(strings.TrimSpace(text)); out != "" {
				edit := WorkspaceEdit{Changes: map[string][]TextEdit{payload.URI: {{Range: payload.Range, NewText: out}}}}
				ca.Edit = &edit
				return ca, true
			}
		} else {
			logging.Logf("lsp ", "codeAction diagnostics llm error: %v", err)
		}
	}
	return ca, false
}

func (s *Server) handleCodeActionResolve(req Request) {
	var ca CodeAction
	if err := json.Unmarshal(req.Params, &ca); err != nil {
		if len(req.ID) != 0 {
			s.reply(req.ID, ca, nil)
		}
		return
	}
	if resolved, ok := s.resolveCodeAction(ca); ok {
		s.reply(req.ID, resolved, nil)
		return
	}
	s.reply(req.ID, ca, nil)
}

// diagnosticsInRange parses the CodeAction context and returns diagnostics
// that overlap the given selection range. If the context is missing or does
// not contain diagnostics, returns an empty slice.
func (s *Server) diagnosticsInRange(ctxRaw json.RawMessage, sel Range) []Diagnostic {
	if len(ctxRaw) == 0 {
		return nil
	}
	var ctx CodeActionContext
	if err := json.Unmarshal(ctxRaw, &ctx); err != nil {
		return nil
	}
	if len(ctx.Diagnostics) == 0 {
		return nil
	}
	out := make([]Diagnostic, 0, len(ctx.Diagnostics))
	for _, d := range ctx.Diagnostics {
		if rangesOverlap(d.Range, sel) {
			out = append(out, d)
		}
	}
	return out
}

// rangesOverlap reports whether two LSP ranges overlap at all.
func rangesOverlap(a, b Range) bool {
	// Normalize ordering
	if greaterPos(a.Start, a.End) {
		a.Start, a.End = a.End, a.Start
	}
	if greaterPos(b.Start, b.End) {
		b.Start, b.End = b.End, b.Start
	}
	// a ends before b starts
	if lessPos(a.End, b.Start) {
		return false
	}
	// b ends before a starts
	if lessPos(b.End, a.Start) {
		return false
	}
	return true
}

func lessPos(p, q Position) bool {
	if p.Line != q.Line {
		return p.Line < q.Line
	}
	return p.Character < q.Character
}

func greaterPos(p, q Position) bool {
	if p.Line != q.Line {
		return p.Line > q.Line
	}
	return p.Character > q.Character
}
