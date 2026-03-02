// Summary: Code Action handlers and helpers split from handlers.go for clarity.
package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/gotest"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/logging"
)

type codeActionPayload struct {
	Type        string       `json:"type"`
	ID          string       `json:"id"`
	URI         string       `json:"uri"`
	Range       Range        `json:"range"`
	Instruction string       `json:"instruction,omitempty"`
	Selection   string       `json:"selection"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// CodeActionHandler builds and resolves code actions for a specific action type.
type CodeActionHandler interface {
	Build(s *Server, p CodeActionParams, selection string) []CodeAction
	Resolve(s *Server, action CodeAction, payload codeActionPayload) (CodeAction, bool)
}

type codeActionHandler struct {
	build   func(s *Server, p CodeActionParams, selection string) []CodeAction
	resolve func(s *Server, action CodeAction, payload codeActionPayload) (CodeAction, bool)
}

func (h codeActionHandler) Build(s *Server, p CodeActionParams, selection string) []CodeAction {
	if h.build == nil {
		return nil
	}
	return h.build(s, p, selection)
}

func (h codeActionHandler) Resolve(s *Server, action CodeAction, payload codeActionPayload) (CodeAction, bool) {
	if h.resolve == nil {
		return action, false
	}
	return h.resolve(s, action, payload)
}

var codeActionBuildOrder = []string{"rewrite", "diagnostics", "document", "go_test", "simplify", "custom"}

func (s *Server) handleCodeAction(req Request) {
	var p CodeActionParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		if len(req.ID) != 0 {
			s.reply(req.ID, []CodeAction{}, nil)
		}
		return
	}
	// Skip code actions for gitignored / extra-pattern-ignored files
	if ignored, reason := s.isFileIgnored(p.TextDocument.URI); ignored {
		logging.Logf("lsp ", "code action skipped: file ignored (%s) uri=%s", reason, p.TextDocument.URI)
		if len(req.ID) != 0 {
			s.reply(req.ID, []CodeAction{}, nil)
		}
		return
	}
	d := s.getDocument(p.TextDocument.URI)
	if d == nil || len(d.lines) == 0 || s.currentLLMClient() == nil {
		if len(req.ID) != 0 {
			s.reply(req.ID, []CodeAction{}, nil)
		}
		return
	}
	sel := extractRangeText(d, p.Range)

	actions := s.buildCodeActions(p, sel)
	if len(req.ID) != 0 {
		s.reply(req.ID, actions, nil)
	}
}

func (s *Server) buildCodeActions(p CodeActionParams, selection string) []CodeAction {
	actions := make([]CodeAction, 0, 8)
	handlers := s.codeActionHandlers()
	for _, key := range codeActionBuildOrder {
		handler, ok := handlers[key]
		if !ok {
			continue
		}
		actions = append(actions, handler.Build(s, p, selection)...)
	}
	return actions
}

// appendCustomActions adds user-defined actions depending on scope and availability.
func (s *Server) appendCustomActions(actions *[]CodeAction, p CodeActionParams, sel string) {
	customs := s.customActions()
	if len(customs) == 0 {
		return
	}
	diags := s.diagnosticsInRange(p.Context, p.Range)
	for _, ca := range customs {
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
			raw, ok := s.marshalCodeActionData(payload)
			if !ok {
				continue
			}
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
		raw, ok := s.marshalCodeActionData(payload)
		if !ok {
			continue
		}
		kind := ca.Kind
		if strings.TrimSpace(kind) == "" {
			kind = "refactor"
		}
		*actions = append(*actions, CodeAction{Title: "Hexai: " + title, Kind: kind, Data: raw})
	}
}

func (s *Server) codeActionHandlers() map[string]CodeActionHandler {
	return map[string]CodeActionHandler{
		"rewrite":     codeActionHandler{build: buildRewriteActions, resolve: resolveRewriteCodeAction},
		"diagnostics": codeActionHandler{build: buildDiagnosticsActions, resolve: resolveDiagnosticsCodeAction},
		"document":    codeActionHandler{build: buildDocumentActions, resolve: resolveDocumentCodeAction},
		"go_test":     codeActionHandler{build: buildGoTestActions, resolve: resolveGoTestCodeAction},
		"simplify":    codeActionHandler{build: buildSimplifyActions, resolve: resolveSimplifyCodeAction},
		"custom":      codeActionHandler{build: buildCustomActions, resolve: resolveCustomCodeAction},
	}
}

func buildRewriteActions(s *Server, p CodeActionParams, selection string) []CodeAction {
	if action := s.buildRewriteCodeAction(p, selection); action != nil {
		return []CodeAction{*action}
	}
	return nil
}

func buildDiagnosticsActions(s *Server, p CodeActionParams, selection string) []CodeAction {
	if action := s.buildDiagnosticsCodeAction(p, selection); action != nil {
		return []CodeAction{*action}
	}
	return nil
}

func buildDocumentActions(s *Server, p CodeActionParams, selection string) []CodeAction {
	if action := s.buildDocumentCodeAction(p, selection); action != nil {
		return []CodeAction{*action}
	}
	return nil
}

func buildGoTestActions(s *Server, p CodeActionParams, _ string) []CodeAction {
	if action := s.buildGoUnitTestCodeAction(p); action != nil {
		return []CodeAction{*action}
	}
	return nil
}

func buildSimplifyActions(s *Server, p CodeActionParams, selection string) []CodeAction {
	if action := s.buildSimplifyCodeAction(p, selection); action != nil {
		return []CodeAction{*action}
	}
	return nil
}

func buildCustomActions(s *Server, p CodeActionParams, selection string) []CodeAction {
	actions := make([]CodeAction, 0, len(s.customActions()))
	s.appendCustomActions(&actions, p, selection)
	return actions
}

func resolveRewriteCodeAction(s *Server, action CodeAction, payload codeActionPayload) (CodeAction, bool) {
	cfg := s.currentConfig()
	sys := cfg.PromptCodeActionRewriteSystem
	user := renderTemplate(cfg.PromptCodeActionRewriteUser, map[string]string{
		"instruction": payload.Instruction,
		"selection":   payload.Selection,
	})
	return s.completeCodeAction(action, payload.URI, payload.Range, sys, user, 20*time.Second)
}

func resolveDiagnosticsCodeAction(s *Server, action CodeAction, payload codeActionPayload) (CodeAction, bool) {
	cfg := s.currentConfig()
	sys := cfg.PromptCodeActionDiagnosticsSystem
	user := renderTemplate(cfg.PromptCodeActionDiagnosticsUser, map[string]string{
		"diagnostics": formatDiagnostics(payload.Diagnostics),
		"selection":   payload.Selection,
	})
	return s.completeCodeAction(action, payload.URI, payload.Range, sys, user, 22*time.Second)
}

func resolveDocumentCodeAction(s *Server, action CodeAction, payload codeActionPayload) (CodeAction, bool) {
	cfg := s.currentConfig()
	sys := cfg.PromptCodeActionDocumentSystem
	user := renderTemplate(cfg.PromptCodeActionDocumentUser, map[string]string{"selection": payload.Selection})
	return s.completeCodeAction(action, payload.URI, payload.Range, sys, user, 20*time.Second)
}

func resolveGoTestCodeAction(s *Server, action CodeAction, payload codeActionPayload) (CodeAction, bool) {
	edit, jumpURI, jumpRange, ok := s.resolveGoTest(payload.URI, payload.Range.Start)
	if !ok {
		return action, false
	}
	action.Edit = &edit
	action.Command = &Command{
		Title:     "Jump to generated test",
		Command:   "hexai.showDocument",
		Arguments: []any{jumpURI, jumpRange},
	}
	s.deferShowDocument(jumpURI, jumpRange)
	return action, true
}

func resolveSimplifyCodeAction(s *Server, action CodeAction, payload codeActionPayload) (CodeAction, bool) {
	cfg := s.currentConfig()
	sys := cfg.PromptCodeActionRewriteSystem
	user := renderTemplate(cfg.PromptCodeActionRewriteUser, map[string]string{
		"instruction": "Simplify and improve the code while preserving behavior. Return only the improved code.",
		"selection":   payload.Selection,
	})
	return s.completeCodeAction(action, payload.URI, payload.Range, sys, user, 20*time.Second)
}

func resolveCustomCodeAction(s *Server, action CodeAction, payload codeActionPayload) (CodeAction, bool) {
	custom := s.customActionByID(payload.ID)
	if custom == nil {
		return action, false
	}
	cfg := s.currentConfig()
	sys := cfg.PromptCodeActionRewriteSystem
	user := renderTemplate(cfg.PromptCodeActionRewriteUser, map[string]string{
		"instruction": payload.Instruction,
		"selection":   payload.Selection,
	})
	if strings.TrimSpace(custom.User) != "" {
		if strings.TrimSpace(custom.System) != "" {
			sys = custom.System
		}
		user = renderTemplate(custom.User, map[string]string{
			"selection":   payload.Selection,
			"diagnostics": strings.TrimSpace(formatCustomDiagnostics(payload.Diagnostics)),
		})
	}
	return s.completeCodeAction(action, payload.URI, payload.Range, sys, user, 20*time.Second)
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
	raw, ok := s.marshalCodeActionData(payload)
	if !ok {
		return nil
	}
	ca := CodeAction{Title: "Hexai: simplify and improve", Kind: "refactor", Data: raw}
	return &ca
}

func (s *Server) buildRewriteCodeAction(p CodeActionParams, sel string) *CodeAction {
	if instr, cleaned := s.instructionFromSelection(sel); strings.TrimSpace(instr) != "" {
		payload := struct {
			Type        string `json:"type"`
			URI         string `json:"uri"`
			Range       Range  `json:"range"`
			Instruction string `json:"instruction"`
			Selection   string `json:"selection"`
		}{Type: "rewrite", URI: p.TextDocument.URI, Range: p.Range, Instruction: instr, Selection: cleaned}
		raw, ok := s.marshalCodeActionData(payload)
		if !ok {
			return nil
		}
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
	raw, ok := s.marshalCodeActionData(payload)
	if !ok {
		return nil
	}
	ca := CodeAction{Title: "Hexai: resolve diagnostics", Kind: "quickfix", Data: raw}
	return &ca
}

func (s *Server) resolveCodeAction(ca CodeAction) (CodeAction, bool) {
	if s.currentLLMClient() == nil || len(ca.Data) == 0 {
		return ca, false
	}
	payload, ok := decodeCodeActionPayload(ca.Data)
	if !ok {
		return ca, false
	}
	handler, found := s.codeActionHandlers()[payload.Type]
	if !found {
		return ca, false
	}
	return handler.Resolve(s, ca, payload)
}

func decodeCodeActionPayload(raw json.RawMessage) (codeActionPayload, bool) {
	var payload codeActionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return codeActionPayload{}, false
	}
	return payload, true
}

func formatDiagnostics(diagnostics []Diagnostic) string {
	var b strings.Builder
	for i, dgn := range diagnostics {
		if dgn.Source != "" {
			fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, dgn.Source, dgn.Message)
			continue
		}
		fmt.Fprintf(&b, "%d. %s\n", i+1, dgn.Message)
	}
	return b.String()
}

func formatCustomDiagnostics(diagnostics []Diagnostic) string {
	var b strings.Builder
	for _, d := range diagnostics {
		fmt.Fprintf(&b, "%s\n", d.Message)
	}
	return b.String()
}

func (s *Server) customActionByID(id string) *appconfig.CustomAction {
	for _, item := range s.customActions() {
		if item.ID == id {
			action := item
			return &action
		}
	}
	return nil
}

func (s *Server) completeCodeAction(ca CodeAction, uri string, rng Range, sys, user string, timeout time.Duration) (CodeAction, bool) {
	ctx, cancel := s.requestTimeoutContext(timeout)
	defer cancel()
	messages := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
	spec := s.buildRequestSpec(surfaceCodeAction)
	if text, err := s.chatWithStats(ctx, surfaceCodeAction, spec, messages); err == nil {
		if out := stripCodeFences(strings.TrimSpace(text)); out != "" {
			edit := WorkspaceEdit{Changes: map[string][]TextEdit{uri: {{Range: rng, NewText: out}}}}
			ca.Edit = &edit
			return ca, true
		}
	} else {
		logging.Logf("lsp ", "codeAction llm error: %v", err)
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

func (s *Server) marshalCodeActionData(payload any) ([]byte, bool) {
	raw, err := json.Marshal(payload)
	if err != nil {
		logging.Logf("lsp ", "code action payload marshal error: %v", err)
		return nil, false
	}
	return raw, true
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
	raw, ok := s.marshalCodeActionData(payload)
	if !ok {
		return nil
	}
	ca := CodeAction{Title: "Hexai: implement unit test", Kind: "quickfix", Data: raw}
	return &ca
}

// buildDocumentCodeAction offers to document the selected code by injecting comments.
func (s *Server) buildDocumentCodeAction(p CodeActionParams, sel string) *CodeAction {
	if s.currentLLMClient() == nil {
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
	raw, ok := s.marshalCodeActionData(payload)
	if !ok {
		return nil
	}
	ca := CodeAction{Title: "Hexai: document code", Kind: "refactor.rewrite", Data: raw}
	return &ca
}

func (s *Server) resolveGoTest(uri string, pos Position) (WorkspaceEdit, string, Range, bool) {
	target, ok := s.resolveGoTestTarget(uri, pos)
	if !ok {
		return WorkspaceEdit{}, "", Range{}, false
	}
	if fileExists(target.testPath) {
		we, jump := s.appendGoTestToExistingFile(target.testURI, target.testFunc)
		return we, target.testURI, jump, true
	}
	we, jump := createGoTestFileWorkspace(target.path, target.testURI, target.pkg, target.testFunc)
	return we, target.testURI, jump, true
}

type goTestTarget struct {
	path     string
	testPath string
	testURI  string
	pkg      string
	testFunc string
}

func (s *Server) resolveGoTestTarget(uri string, pos Position) (goTestTarget, bool) {
	path := strings.TrimPrefix(uri, "file://")
	if !isNonTestGoFile(path) {
		return goTestTarget{}, false
	}
	_, lines := s.loadFileText(uri)
	if len(lines) == 0 {
		return goTestTarget{}, false
	}
	fnStart, fnEnd := findGoFunctionAtLine(lines, pos.Line)
	if fnStart < 0 || fnEnd < fnStart {
		return goTestTarget{}, false
	}
	funcCode := strings.Join(lines[fnStart:fnEnd+1], "\n")
	testFunc := s.generateGoTestFunction(funcCode)
	if strings.TrimSpace(testFunc) == "" {
		return goTestTarget{}, false
	}
	testPath := strings.TrimSuffix(path, ".go") + "_test.go"
	return goTestTarget{
		path:     path,
		testPath: testPath,
		testURI:  "file://" + testPath,
		pkg:      parseGoPackageName(lines),
		testFunc: testFunc,
	}, true
}

func isNonTestGoFile(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}

func (s *Server) appendGoTestToExistingFile(testURI string, testFunc string) (WorkspaceEdit, Range) {
	_, tLines := s.loadFileText(testURI)
	lineIdx, col := appendInsertPosition(tLines)
	insert := appendInsertText(tLines, testFunc)
	edit := TextEdit{
		Range:   Range{Start: Position{Line: lineIdx, Character: col}, End: Position{Line: lineIdx, Character: col}},
		NewText: insert,
	}
	we := WorkspaceEdit{Changes: map[string][]TextEdit{testURI: {edit}}}
	startLine := lineIdx + strings.Count(insert[:len(insert)-len(testFunc)], "\n")
	jump := Range{Start: Position{Line: startLine, Character: 0}, End: Position{Line: startLine, Character: 0}}
	return we, jump
}

func appendInsertPosition(lines []string) (int, int) {
	if len(lines) == 0 {
		return 0, 0
	}
	lineIdx := len(lines) - 1
	return lineIdx, len(lines[lineIdx])
}

func appendInsertText(lines []string, testFunc string) string {
	prefix := "\n\n"
	if len(lines) > 0 && strings.HasSuffix(strings.Join(lines, "\n"), "\n\n") {
		prefix = ""
	}
	return prefix + testFunc
}

func createGoTestFileWorkspace(path string, testURI string, pkg string, testFunc string) (WorkspaceEdit, Range) {
	content := newGoTestFileContent(path, pkg, testFunc)
	create := CreateFile{Kind: "create", URI: testURI}
	edit := TextEdit{Range: Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: 0}}, NewText: content}
	tde := TextDocumentEdit{TextDocument: VersionedTextDocumentIdentifier{URI: testURI}, Edits: []TextEdit{edit}}
	we := WorkspaceEdit{DocumentChanges: []any{create, tde}}
	startLine := findFirstTestStartLine(content)
	jump := Range{Start: Position{Line: startLine, Character: 0}, End: Position{Line: startLine, Character: 0}}
	return we, jump
}

func newGoTestFileContent(path string, pkg string, testFunc string) string {
	if pkg == "" {
		pkg = filepath.Base(filepath.Dir(path))
	}
	var content strings.Builder
	content.WriteString("package ")
	content.WriteString(pkg)
	content.WriteString("\n\n")
	content.WriteString("import (\n\t\"testing\"\n)\n\n")
	content.WriteString(testFunc)
	return content.String()
}

func findFirstTestStartLine(content string) int {
	idx := strings.Index(content, "func Test")
	if idx <= 0 {
		return 0
	}
	return strings.Count(content[:idx], "\n")
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
	return gotest.ParsePackageName(lines)
}

// findGoFunctionAtLine finds the function enclosing or preceding line idx. Returns start and end line indexes.
func findGoFunctionAtLine(lines []string, idx int) (int, int) {
	return gotest.FindFunctionAtLine(lines, idx)
}

// generateGoTestFunction uses LLM to produce a test function; falls back to a stub when unavailable.
func (s *Server) generateGoTestFunction(funcCode string) string {
	spec := s.buildRequestSpec(surfaceCodeAction)
	cfg := s.currentConfig()
	sys := cfg.PromptCodeActionGoTestSystem
	user := renderTemplate(cfg.PromptCodeActionGoTestUser, map[string]string{"function": funcCode})
	ctx, cancel := s.requestTimeoutContext(18 * time.Second)
	defer cancel()
	messages := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
	if out, err := s.chatWithStats(ctx, surfaceCodeAction, spec, messages); err == nil {
		cleaned := strings.TrimSpace(stripCodeFences(out))
		if cleaned != "" {
			return cleaned
		}
	} else {
		logging.Logf("lsp ", "codeAction go_test llm error: %v", err)
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
	return gotest.DeriveFuncName(code)
}

func exportName(name string) string {
	return gotest.ExportName(name)
}
