// Summary: Code Action handlers and helpers split from handlers.go for clarity.
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/logging"
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

	actions := make([]CodeAction, 0, 8)
	if a := s.buildRewriteCodeAction(p, sel); a != nil {
		actions = append(actions, *a)
	}
	if a := s.buildDiagnosticsCodeAction(p, sel); a != nil {
		actions = append(actions, *a)
	}
	if a := s.buildDocumentCodeAction(p, sel); a != nil {
		actions = append(actions, *a)
	}
	if a := s.buildGoUnitTestCodeAction(p); a != nil {
		actions = append(actions, *a)
	}
	if a := s.buildSimplifyCodeAction(p, sel); a != nil {
		actions = append(actions, *a)
	}
	// Custom actions from config
	s.appendCustomActions(&actions, p, sel)
	if len(req.ID) != 0 {
		s.reply(req.ID, actions, nil)
	}
}

// appendCustomActions adds user-defined actions depending on scope and availability.
func (s *Server) appendCustomActions(actions *[]CodeAction, p CodeActionParams, sel string) {
	if len(s.customActions) == 0 {
		return
	}
	diags := s.diagnosticsInRange(p.Context, p.Range)
	for _, ca := range s.customActions {
		title := strings.TrimSpace(ca.Title)
		if title == "" {
			continue
		}
		scope := strings.TrimSpace(strings.ToLower(ca.Scope))
		if scope == "diagnostics" {
			if len(diags) == 0 {
				continue
			}
			payload := struct {
				Type        string       `json:"type"`
				ID          string       `json:"id"`
				URI         string       `json:"uri"`
				Range       Range        `json:"range"`
				Selection   string       `json:"selection"`
				Diagnostics []Diagnostic `json:"diagnostics"`
			}{Type: "custom", ID: ca.ID, URI: p.TextDocument.URI, Range: p.Range, Selection: sel, Diagnostics: diags}
			raw, _ := json.Marshal(payload)
			kind := ca.Kind
			if strings.TrimSpace(kind) == "" {
				kind = "quickfix"
			}
			*actions = append(*actions, CodeAction{Title: "Hexai: " + title, Kind: kind, Data: raw})
			continue
		}
		// default: selection
		if strings.TrimSpace(sel) == "" {
			continue
		}
		payload := struct {
			Type      string `json:"type"`
			ID        string `json:"id"`
			URI       string `json:"uri"`
			Range     Range  `json:"range"`
			Selection string `json:"selection"`
		}{Type: "custom", ID: ca.ID, URI: p.TextDocument.URI, Range: p.Range, Selection: sel}
		raw, _ := json.Marshal(payload)
		kind := ca.Kind
		if strings.TrimSpace(kind) == "" {
			kind = "refactor"
		}
		*actions = append(*actions, CodeAction{Title: "Hexai: " + title, Kind: kind, Data: raw})
	}
}

func (s *Server) buildSimplifyCodeAction(p CodeActionParams, sel string) *CodeAction {
	if strings.TrimSpace(sel) == "" {
		return nil
	}
	payload := struct {
		Type      string `json:"type"`
		URI       string `json:"uri"`
		Range     Range  `json:"range"`
		Selection string `json:"selection"`
	}{Type: "simplify", URI: p.TextDocument.URI, Range: p.Range, Selection: sel}
	raw, _ := json.Marshal(payload)
	ca := CodeAction{Title: "Hexai: simplify and improve", Kind: "refactor", Data: raw}
	return &ca
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
		ID          string       `json:"id"`
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
		sys := s.promptRewriteSystem
		user := renderTemplate(s.promptRewriteUser, map[string]string{"instruction": payload.Instruction, "selection": payload.Selection})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		messages := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
		opts := s.llmRequestOpts()
		if text, err := s.chatWithStats(ctx, messages, opts...); err == nil {
			if out := stripCodeFences(strings.TrimSpace(text)); out != "" {
				edit := WorkspaceEdit{Changes: map[string][]TextEdit{payload.URI: {{Range: payload.Range, NewText: out}}}}
				ca.Edit = &edit
				return ca, true
			}
		} else {
			logging.Logf("lsp ", "codeAction rewrite llm error: %v", err)
		}
	case "diagnostics":
		sys := s.promptDiagnosticsSystem
		var b strings.Builder
		for i, dgn := range payload.Diagnostics {
			if dgn.Source != "" {
				fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, dgn.Source, dgn.Message)
			} else {
				fmt.Fprintf(&b, "%d. %s\n", i+1, dgn.Message)
			}
		}
		diagList := b.String()
		user := renderTemplate(s.promptDiagnosticsUser, map[string]string{"diagnostics": diagList, "selection": payload.Selection})
		ctx, cancel := context.WithTimeout(context.Background(), 22*time.Second)
		defer cancel()
		messages := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
		opts := s.llmRequestOpts()
		if text, err := s.chatWithStats(ctx, messages, opts...); err == nil {
			if out := stripCodeFences(strings.TrimSpace(text)); out != "" {
				edit := WorkspaceEdit{Changes: map[string][]TextEdit{payload.URI: {{Range: payload.Range, NewText: out}}}}
				ca.Edit = &edit
				return ca, true
			}
		} else {
			logging.Logf("lsp ", "codeAction diagnostics llm error: %v", err)
		}
	case "document":
		sys := s.promptDocumentSystem
		user := renderTemplate(s.promptDocumentUser, map[string]string{"selection": payload.Selection})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		messages := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
		opts := s.llmRequestOpts()
		if text, err := s.chatWithStats(ctx, messages, opts...); err == nil {
			if out := stripCodeFences(strings.TrimSpace(text)); out != "" {
				edit := WorkspaceEdit{Changes: map[string][]TextEdit{payload.URI: {{Range: payload.Range, NewText: out}}}}
				ca.Edit = &edit
				return ca, true
			}
		} else {
			logging.Logf("lsp ", "codeAction document llm error: %v", err)
		}
	case "go_test":
		if edit, jumpURI, jumpRange, ok := s.resolveGoTest(payload.URI, payload.Range.Start); ok {
			ca.Edit = &edit
			// After edit is applied, ask client to jump to new test function
			ca.Command = &Command{Title: "Jump to generated test", Command: "hexai.showDocument", Arguments: []any{jumpURI, jumpRange}}
			// Also send a server-initiated showDocument shortly after resolve to cover
			// clients that do not execute commands from code actions.
			s.deferShowDocument(jumpURI, jumpRange)
			return ca, true
		}
	case "simplify":
		sys := s.promptRewriteSystem
		// Reuse rewrite user template with a fixed instruction
		user := renderTemplate(s.promptRewriteUser, map[string]string{"instruction": "Simplify and improve the code while preserving behavior. Return only the improved code.", "selection": payload.Selection})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		messages := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
		opts := s.llmRequestOpts()
		if text, err := s.chatWithStats(ctx, messages, opts...); err == nil {
			if out := stripCodeFences(strings.TrimSpace(text)); out != "" {
				edit := WorkspaceEdit{Changes: map[string][]TextEdit{payload.URI: {{Range: payload.Range, NewText: out}}}}
				ca.Edit = &edit
				return ca, true
			}
		} else {
			logging.Logf("lsp ", "codeAction simplify llm error: %v", err)
		}
	case "custom":
		// Lookup action by ID
		var action *CustomAction
		for i := range s.customActions {
			if s.customActions[i].ID == payload.ID {
				action = &s.customActions[i]
				break
			}
		}
		if action == nil {
			return ca, false
		}
		// Build messages
		var sys, user string
		if strings.TrimSpace(action.User) != "" {
			if strings.TrimSpace(action.System) != "" {
				sys = action.System
			} else {
				sys = s.promptRewriteSystem
			}
			var diagList string
			if len(payload.Diagnostics) > 0 {
				var b strings.Builder
				for i, dgn := range payload.Diagnostics {
					if dgn.Source != "" {
						fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, dgn.Source, dgn.Message)
					} else {
						fmt.Fprintf(&b, "%d. %s\n", i+1, dgn.Message)
					}
				}
				diagList = b.String()
			}
			user = renderTemplate(action.User, map[string]string{"selection": payload.Selection, "diagnostics": diagList})
		} else {
			// Use rewrite templates with fixed instruction
			sys = s.promptRewriteSystem
			user = renderTemplate(s.promptRewriteUser, map[string]string{"instruction": action.Instruction, "selection": payload.Selection})
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		messages := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
		opts := s.llmRequestOpts()
		if text, err := s.chatWithStats(ctx, messages, opts...); err == nil {
			if out := stripCodeFences(strings.TrimSpace(text)); out != "" {
				edit := WorkspaceEdit{Changes: map[string][]TextEdit{payload.URI: {{Range: payload.Range, NewText: out}}}}
				ca.Edit = &edit
				return ca, true
			}
		} else {
			logging.Logf("lsp ", "codeAction custom id=%s llm error: %v", action.ID, err)
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

// --- Go unit test code action ---

func (s *Server) buildGoUnitTestCodeAction(p CodeActionParams) *CodeAction {
	uri := p.TextDocument.URI
	if uri == "" || !strings.HasSuffix(strings.TrimPrefix(uri, "file://"), ".go") {
		return nil
	}
	// Skip if already a _test.go file
	if strings.HasSuffix(strings.TrimPrefix(uri, "file://"), "_test.go") {
		return nil
	}
	// Heuristic: only offer when a function context is found above the cursor
	_, _, _, funcCtx := s.lineContext(uri, p.Range.Start)
	if !strings.Contains(funcCtx, "func ") {
		return nil
	}
	payload := struct {
		Type  string `json:"type"`
		URI   string `json:"uri"`
		Range Range  `json:"range"`
	}{Type: "go_test", URI: uri, Range: p.Range}
	raw, _ := json.Marshal(payload)
	ca := CodeAction{Title: "Hexai: implement unit test", Kind: "quickfix", Data: raw}
	return &ca
}

// buildDocumentCodeAction offers to document the selected code by injecting comments.
func (s *Server) buildDocumentCodeAction(p CodeActionParams, sel string) *CodeAction {
	if s.llmClient == nil {
		return nil
	}
	if strings.TrimSpace(sel) == "" {
		return nil
	}
	payload := struct {
		Type      string `json:"type"`
		URI       string `json:"uri"`
		Range     Range  `json:"range"`
		Selection string `json:"selection"`
	}{Type: "document", URI: p.TextDocument.URI, Range: p.Range, Selection: sel}
	raw, _ := json.Marshal(payload)
	ca := CodeAction{Title: "Hexai: document code", Kind: "refactor.rewrite", Data: raw}
	return &ca
}

func (s *Server) resolveGoTest(uri string, pos Position) (WorkspaceEdit, string, Range, bool) {
	path := strings.TrimPrefix(uri, "file://")
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return WorkspaceEdit{}, "", Range{}, false
	}
	// Load source text
	_, lines := s.loadFileText(uri)
	if len(lines) == 0 {
		return WorkspaceEdit{}, "", Range{}, false
	}
	pkg := parseGoPackageName(lines)
	fnStart, fnEnd := findGoFunctionAtLine(lines, pos.Line)
	if fnStart < 0 || fnEnd < fnStart {
		return WorkspaceEdit{}, "", Range{}, false
	}
	funcCode := strings.Join(lines[fnStart:fnEnd+1], "\n")
	testFunc := s.generateGoTestFunction(funcCode)
	if strings.TrimSpace(testFunc) == "" {
		return WorkspaceEdit{}, "", Range{}, false
	}
	// Determine test file target
	testPath := strings.TrimSuffix(path, ".go") + "_test.go"
	testURI := "file://" + testPath

	// If test file exists, append test at EOF; otherwise, create a new file with package+import
	if fileExists(testPath) {
		// Build an insertion at end of file
		_, tLines := s.loadFileText(testURI)
		// Fallback when not open and cannot read: still insert at line 0
		lineIdx := 0
		col := 0
		if len(tLines) > 0 {
			lineIdx = len(tLines) - 1
			col = len(tLines[lineIdx])
		}
		var b strings.Builder
		// Ensure at least two newlines before the new test
		if len(tLines) == 0 || (len(tLines) > 0 && !strings.HasSuffix(strings.Join(tLines, "\n"), "\n\n")) {
			b.WriteString("\n\n")
		}
		b.WriteString(testFunc)
		insert := b.String()
		edit := TextEdit{Range: Range{Start: Position{Line: lineIdx, Character: col}, End: Position{Line: lineIdx, Character: col}}, NewText: insert}
		we := WorkspaceEdit{Changes: map[string][]TextEdit{testURI: {edit}}}
		// Compute jump range start
		// Count how many prefix newlines added before the test function
		prefixNL := 0
		if strings.HasPrefix(insert, "\n\n") {
			prefixNL = 2
		}
		startLine := lineIdx + prefixNL
		// If we inserted with two newlines and last line wasn't blank, first newline moves to next line
		if prefixNL > 0 {
			startLine = lineIdx + prefixNL
		}
		jump := Range{Start: Position{Line: startLine, Character: 0}, End: Position{Line: startLine, Character: 0}}
		return we, testURI, jump, true
	}
	// Create new file content
	var content strings.Builder
	if pkg == "" {
		pkg = filepath.Base(filepath.Dir(path))
	}
	content.WriteString("package ")
	content.WriteString(pkg)
	content.WriteString("\n\n")
	content.WriteString("import (\n\t\"testing\"\n)\n\n")
	content.WriteString(testFunc)
	full := content.String()
	// Use documentChanges with create + full content insert
	create := CreateFile{Kind: "create", URI: testURI}
	tde := TextDocumentEdit{TextDocument: VersionedTextDocumentIdentifier{URI: testURI}, Edits: []TextEdit{{Range: Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: 0}}, NewText: full}}}
	we := WorkspaceEdit{DocumentChanges: []any{create, tde}}
	// Find start line of first test function
	// Count lines before the substring "func Test"
	pre := content.String()
	idx := strings.Index(pre, "func Test")
	startLine := 0
	if idx > 0 {
		before := pre[:idx]
		startLine = strings.Count(before, "\n")
	}
	jump := Range{Start: Position{Line: startLine, Character: 0}, End: Position{Line: startLine, Character: 0}}
	return we, testURI, jump, true
}

// loadFileText returns the file content and lines. It prefers the open document; otherwise reads from disk.
func (s *Server) loadFileText(uri string) (string, []string) {
	if d := s.getDocument(uri); d != nil {
		return d.text, append([]string{}, d.lines...)
	}
	path := strings.TrimPrefix(uri, "file://")
	b, err := os.ReadFile(path)
	if err != nil {
		return "", nil
	}
	txt := string(b)
	return txt, splitLines(txt)
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

// parseGoPackageName returns the package name from file lines, or empty if not found.
func parseGoPackageName(lines []string) string {
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "package ") {
			name := strings.TrimSpace(strings.TrimPrefix(t, "package "))
			// strip inline comments
			if i := strings.Index(name, " "); i >= 0 {
				name = name[:i]
			}
			if i := strings.Index(name, "\t"); i >= 0 {
				name = name[:i]
			}
			if i := strings.Index(name, "//"); i >= 0 {
				name = strings.TrimSpace(name[:i])
			}
			return name
		}
	}
	return ""
}

// findGoFunctionAtLine finds the function enclosing or preceding line idx. Returns start and end line indexes.
func findGoFunctionAtLine(lines []string, idx int) (int, int) {
	if idx < 0 {
		idx = 0
	}
	if idx >= len(lines) {
		idx = len(lines) - 1
	}
	// find signature start
	start := -1
	for i := idx; i >= 0; i-- {
		if strings.Contains(lines[i], "func ") {
			start = i
			break
		}
		if strings.Contains(lines[i], "}") {
			break
		}
	}
	if start == -1 {
		return -1, -1
	}
	// find first '{'
	depth := 0
	seenOpen := false
	for i := start; i < len(lines); i++ {
		ln := lines[i]
		for j := 0; j < len(ln); j++ {
			switch ln[j] {
			case '{':
				depth++
				seenOpen = true
			case '}':
				if depth > 0 {
					depth--
				}
				if seenOpen && depth == 0 {
					return start, i
				}
			}
		}
	}
	// if never saw '{', assume single-line prototype; return that line
	if !seenOpen {
		return start, start
	}
	return start, -1
}

// generateGoTestFunction uses LLM to produce a test function; falls back to a stub when unavailable.
func (s *Server) generateGoTestFunction(funcCode string) string {
	if s.llmClient != nil {
		sys := s.promptGoTestSystem
		user := renderTemplate(s.promptGoTestUser, map[string]string{"function": funcCode})
		ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
		defer cancel()
		messages := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
		opts := s.llmRequestOpts()
		if out, err := s.chatWithStats(ctx, messages, opts...); err == nil {
			cleaned := strings.TrimSpace(stripCodeFences(out))
			if cleaned != "" {
				return cleaned
			}
		} else {
			logging.Logf("lsp ", "codeAction go_test llm error: %v", err)
		}
	}
	// Fallback stub
	name := deriveGoFuncName(funcCode)
	if name == "" {
		name = "Function"
	}
	return fmt.Sprintf("func Test%s(t *testing.T) {\n\t// TODO: implement tests for %s\n}\n", exportName(name), name)
}

// deriveGoFuncName extracts function or method name from code.
func deriveGoFuncName(code string) string {
	// look for line starting with func
	line := firstLine(code)
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "func ") {
		return ""
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "func "))
	// method receiver
	if strings.HasPrefix(rest, "(") {
		// find ")"
		if i := strings.Index(rest, ")"); i >= 0 && i+1 < len(rest) {
			rest = strings.TrimSpace(rest[i+1:])
		}
	}
	// now rest should start with Name(
	if i := strings.Index(rest, "("); i > 0 {
		return strings.TrimSpace(rest[:i])
	}
	return ""
}

func exportName(name string) string {
	if name == "" {
		return name
	}
	r := []rune(name)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] = r[0] - ('a' - 'A')
	}
	return string(r)
}
