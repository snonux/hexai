// Summary: LSP JSON-RPC handlers; implements core methods and integrates with the LLM client when enabled.
// Not yet reviewed by a human
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"hexai/internal"
	"hexai/internal/llm"
	"hexai/internal/logging"
	"os"
	"strings"
	"time"
)

func (s *Server) handle(req Request) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		s.handleInitialized()
	case "shutdown":
		s.handleShutdown(req)
	case "exit":
		s.handleExit()
	case "textDocument/didOpen":
		s.handleDidOpen(req)
	case "textDocument/didChange":
		s.handleDidChange(req)
	case "textDocument/didClose":
		s.handleDidClose(req)
	case "textDocument/completion":
		s.handleCompletion(req)
	case "textDocument/codeAction":
		s.handleCodeAction(req)
	default:
		if len(req.ID) != 0 {
			s.reply(req.ID, nil, &RespError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)})
		}
	}
}

func (s *Server) handleInitialize(req Request) {
	version := internal.Version
	if s.llmClient != nil {
		version = version + " [" + s.llmClient.Name() + ":" + s.llmClient.DefaultModel() + "]"
	}
	res := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: 1, // 1 = TextDocumentSyncKindFull
			CompletionProvider: &CompletionOptions{
				ResolveProvider:   false,
				TriggerCharacters: s.triggerChars,
			},
			CodeActionProvider: true,
		},
		ServerInfo: &ServerInfo{Name: "hexai", Version: version},
	}
	s.reply(req.ID, res, nil)
}

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
		sys := "You are a precise code refactoring engine. Rewrite the given code strictly according to the instruction. Return only the updated code with no prose or backticks. Preserve formatting where reasonable."
		user := fmt.Sprintf("Instruction: %s\n\nSelected code to transform:\n%s", instr, cleaned)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		messages := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}
		opts := s.llmRequestOpts()
		if text, err := s.llmClient.Chat(ctx, messages, opts...); err == nil {
			if out := stripCodeFences(strings.TrimSpace(text)); out != "" {
				edit := WorkspaceEdit{Changes: map[string][]TextEdit{p.TextDocument.URI: {{Range: p.Range, NewText: out}}}}
				ca := CodeAction{Title: "Hexai: rewrite selection", Kind: "refactor.rewrite", Edit: &edit}
				return &ca
			}
		} else {
			logging.Logf("lsp ", "codeAction rewrite llm error: %v", err)
		}
	}
	return nil
}

func (s *Server) buildDiagnosticsCodeAction(p CodeActionParams, sel string) *CodeAction {
	diags := s.diagnosticsInRange(p.Context, p.Range)
	if len(diags) == 0 {
		return nil
	}
	sys := "You are a precise code fixer. Resolve the given diagnostics by editing only the selected code. Return only the corrected code with no prose or backticks. Keep behavior and style, and avoid unrelated changes."
	var b strings.Builder
	b.WriteString("Diagnostics to resolve (selection only):\n")
	for i, dgn := range diags {
		if dgn.Source != "" {
			fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, dgn.Source, dgn.Message)
		} else {
			fmt.Fprintf(&b, "%d. %s\n", i+1, dgn.Message)
		}
	}
	b.WriteString("\nSelected code:\n")
	b.WriteString(sel)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	messages := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: b.String()}}
	opts := s.llmRequestOpts()
	if text, err := s.llmClient.Chat(ctx, messages, opts...); err == nil {
		if out := stripCodeFences(strings.TrimSpace(text)); out != "" {
			edit := WorkspaceEdit{Changes: map[string][]TextEdit{p.TextDocument.URI: {{Range: p.Range, NewText: out}}}}
			ca := CodeAction{Title: "Hexai: resolve diagnostics", Kind: "quickfix", Edit: &edit}
			return &ca
		}
	} else {
		logging.Logf("lsp ", "codeAction diagnostics llm error: %v", err)
	}
	return nil
}

func (s *Server) llmRequestOpts() []llm.RequestOption {
	opts := []llm.RequestOption{llm.WithMaxTokens(s.maxTokens)}
	if s.codingTemperature != nil {
		opts = append(opts, llm.WithTemperature(*s.codingTemperature))
	}
	return opts
}

// instructionFromSelection extracts the first instruction from selection text.
// Preference order on each line: strict ;text; marker (no inner spaces), then
// a line comment (//, #, --). Returns the instruction string and the selection
// text cleaned of the matched instruction marker or comment.
func instructionFromSelection(sel string) (string, string) {
	lines := splitLines(sel)
	for idx, line := range lines {
		if instr, cleaned, ok := findFirstInstructionInLine(line); ok && strings.TrimSpace(instr) != "" {
			lines[idx] = cleaned
			return instr, strings.Join(lines, "\n")
		}
	}
	return "", sel
}

// findFirstInstructionInLine returns the earliest instruction marker on the
// line and the line with that marker removed. Supported markers, ordered by
// earliest byte offset in the line:
// - ;text; (strict, no space after first ';' or before last ';')
// - /* text */ (single-line only)
// - <!-- text --> (single-line only)
// - // text
// - # text
// - -- text
func findFirstInstructionInLine(line string) (instr string, cleaned string, ok bool) {
	type cand struct {
		start, end int
		text       string
	}
	cands := []cand{}
	if t, l, r, ok := findStrictSemicolonTag(line); ok {
		cands = append(cands, cand{start: l, end: r, text: t})
	}
	if i := strings.Index(line, "/*"); i >= 0 {
		if j := strings.Index(line[i+2:], "*/"); j >= 0 {
			start := i
			end := i + 2 + j + 2
			text := strings.TrimSpace(line[i+2 : i+2+j])
			cands = append(cands, cand{start: start, end: end, text: text})
		}
	}
	if i := strings.Index(line, "<!--"); i >= 0 {
		if j := strings.Index(line[i+4:], "-->"); j >= 0 {
			start := i
			end := i + 4 + j + 3
			text := strings.TrimSpace(line[i+4 : i+4+j])
			cands = append(cands, cand{start: start, end: end, text: text})
		}
	}
	if i := strings.Index(line, "//"); i >= 0 {
		cands = append(cands, cand{start: i, end: len(line), text: strings.TrimSpace(line[i+2:])})
	}
	if i := strings.Index(line, "#"); i >= 0 {
		cands = append(cands, cand{start: i, end: len(line), text: strings.TrimSpace(line[i+1:])})
	}
	if i := strings.Index(line, "--"); i >= 0 {
		cands = append(cands, cand{start: i, end: len(line), text: strings.TrimSpace(line[i+2:])})
	}
	if len(cands) == 0 {
		return "", line, false
	}
	// pick earliest start index
	best := cands[0]
	for _, c := range cands[1:] {
		if c.start >= 0 && (best.start < 0 || c.start < best.start) {
			best = c
		}
	}
	cleaned = strings.TrimRight(line[:best.start]+line[best.end:], " \t")
	return best.text, cleaned, true
}

// findStrictSemicolonTag finds ;text; with no space after first ';' and no space
// before the last ';' on the given line. Returns the text between semicolons,
// the start index of the opening ';', the end index just after the closing ';',
// and whether it was found.
func findStrictSemicolonTag(line string) (string, int, int, bool) {
	pos := 0
	for pos < len(line) {
		j := strings.Index(line[pos:], ";")
		if j < 0 {
			return "", 0, 0, false
		}
		j += pos
		// ensure single ';' (not ';;') and non-space after
		if j+1 >= len(line) || line[j+1] == ';' || line[j+1] == ' ' {
			pos = j + 1
			continue
		}
		k := strings.Index(line[j+1:], ";")
		if k < 0 {
			return "", 0, 0, false
		}
		closeIdx := j + 1 + k
		if closeIdx-1 < 0 || line[closeIdx-1] == ' ' {
			pos = closeIdx + 1
			continue
		}
		inner := strings.TrimSpace(line[j+1 : closeIdx])
		if inner == "" {
			pos = closeIdx + 1
			continue
		}
		end := closeIdx + 1
		return inner, j, end, true
	}
	return "", 0, 0, false
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

// extractRangeText returns the exact text within the given document range.
func extractRangeText(d *document, r Range) string {
	if r.Start.Line == r.End.Line {
		line := d.lines[r.Start.Line]
		if r.Start.Character < 0 {
			r.Start.Character = 0
		}
		if r.End.Character > len(line) {
			r.End.Character = len(line)
		}
		if r.Start.Character > r.End.Character {
			return ""
		}
		return line[r.Start.Character:r.End.Character]
	}
	var b strings.Builder
	// first line
	first := d.lines[r.Start.Line]
	if r.Start.Character < 0 {
		r.Start.Character = 0
	}
	if r.Start.Character > len(first) {
		r.Start.Character = len(first)
	}
	b.WriteString(first[r.Start.Character:])
	b.WriteString("\n")
	// middle lines
	for i := r.Start.Line + 1; i < r.End.Line; i++ {
		b.WriteString(d.lines[i])
		if i+1 <= r.End.Line {
			b.WriteString("\n")
		}
	}
	// last line
	last := d.lines[r.End.Line]
	if r.End.Character < 0 {
		r.End.Character = 0
	}
	if r.End.Character > len(last) {
		r.End.Character = len(last)
	}
	b.WriteString(last[:r.End.Character])
	return b.String()
}

func (s *Server) handleInitialized() {
	logging.Logf("lsp ", "client initialized")
}

func (s *Server) handleShutdown(req Request) {
	s.reply(req.ID, nil, nil)
}

func (s *Server) handleExit() {
	s.exited = true
	os.Exit(0)
}

func (s *Server) handleDidOpen(req Request) {
	var p DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params, &p); err == nil {
		s.setDocument(p.TextDocument.URI, p.TextDocument.Text)
		s.markActivity()
	}
}

func (s *Server) handleDidChange(req Request) {
	var p DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params, &p); err == nil {
		if len(p.ContentChanges) > 0 {
			s.setDocument(p.TextDocument.URI, p.ContentChanges[len(p.ContentChanges)-1].Text)
		}
		s.markActivity()
	}
}

func (s *Server) handleDidClose(req Request) {
	var p DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params, &p); err == nil {
		s.deleteDocument(p.TextDocument.URI)
		s.markActivity()
	}
}

func (s *Server) handleCompletion(req Request) {
	var p CompletionParams
	var docStr string
	if err := json.Unmarshal(req.Params, &p); err == nil {
		above, current, below, funcCtx := s.lineContext(p.TextDocument.URI, p.Position)
		docStr = s.buildDocString(p, above, current, below, funcCtx)
		if s.logContext {
			s.logCompletionContext(p, above, current, below, funcCtx)
		}
		if s.llmClient != nil {
			newFunc := s.isDefiningNewFunction(p.TextDocument.URI, p.Position)
			extra, has := s.buildAdditionalContext(newFunc, p.TextDocument.URI, p.Position)
			items, ok := s.tryLLMCompletion(p, above, current, below, funcCtx, docStr, has, extra)
			if ok {
				s.reply(req.ID, CompletionList{IsIncomplete: false, Items: items}, nil)
				return
			}
		}
	}
	items := s.fallbackCompletionItems(docStr)
	s.reply(req.ID, CompletionList{IsIncomplete: false, Items: items}, nil)
}

func (s *Server) reply(id json.RawMessage, result any, err *RespError) {
	resp := Response{JSONRPC: "2.0", ID: id, Result: result, Error: err}
	s.writeMessage(resp)
}

// --- completion helpers ---

func (s *Server) buildDocString(p CompletionParams, above, current, below, funcCtx string) string {
	return fmt.Sprintf("file: %s\nline: %d\nabove: %s\ncurrent: %s\nbelow: %s\nfunction: %s",
		p.TextDocument.URI, p.Position.Line, trimLen(above), trimLen(current), trimLen(below), trimLen(funcCtx))
}

func (s *Server) logCompletionContext(p CompletionParams, above, current, below, funcCtx string) {
	logging.Logf("lsp ", "completion ctx uri=%s line=%d char=%d above=%q current=%q below=%q function=%q",
		p.TextDocument.URI, p.Position.Line, p.Position.Character, trimLen(above), trimLen(current), trimLen(below), trimLen(funcCtx))
}

func (s *Server) tryLLMCompletion(p CompletionParams, above, current, below, funcCtx, docStr string, hasExtra bool, extraText string) ([]CompletionItem, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	inParams := inParamList(current, p.Position.Character)
	sysPrompt, userPrompt := buildPrompts(inParams, p, above, current, below, funcCtx)
	messages := []llm.Message{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: userPrompt},
	}
	if hasExtra && extraText != "" {
		messages = append(messages, llm.Message{Role: "user", Content: "Additional context:\n" + extraText})
	}

	// Compute total sent context size (sum of message contents)
	var sentSize int
	for _, m := range messages {
		sentSize += len(m.Content)
	}
	s.incSentCounters(sentSize)

	opts := []llm.RequestOption{llm.WithMaxTokens(s.maxTokens)}
	if s.codingTemperature != nil {
		opts = append(opts, llm.WithTemperature(*s.codingTemperature))
	}
	text, err := s.llmClient.Chat(ctx, messages, opts...)
	if err != nil {
		logging.Logf("lsp ", "llm completion error: %v", err)
		// Log updated averages after this request (even if failed)
		s.logLLMStats()
		return nil, false
	}
	// Update response counters (received)
	s.incRecvCounters(len(text))
	s.logLLMStats()
	cleaned := stripCodeFences(strings.TrimSpace(text))
	if cleaned != "" {
		cleaned = stripDuplicateAssignmentPrefix(current[:p.Position.Character], cleaned)
	}
	if cleaned == "" {
		return nil, false
	}

	return s.makeCompletionItems(cleaned, inParams, current, p, docStr), true
}

func (s *Server) makeCompletionItems(cleaned string, inParams bool, current string, p CompletionParams, docStr string) []CompletionItem {
	te, filter := computeTextEditAndFilter(cleaned, inParams, current, p)
	rm := s.collectPromptRemovalEdits(p.TextDocument.URI)
	label := labelForCompletion(cleaned, filter)
	detail := "Hexai LLM completion"
	if s.llmClient != nil {
		detail = "Hexai " + s.llmClient.Name() + ":" + s.llmClient.DefaultModel()
	}
	return []CompletionItem{{
		Label:               label,
		Kind:                1,
		Detail:              detail,
		InsertTextFormat:    1,
		FilterText:          strings.TrimLeft(filter, " \t"),
		TextEdit:            te,
		AdditionalTextEdits: rm,
		SortText:            "0000",
		Documentation:       docStr,
	}}
}

// small helpers to keep tryLLMCompletion short
func (s *Server) incSentCounters(n int) {
	s.mu.Lock()
	s.llmReqTotal++
	s.llmSentBytesTotal += int64(n)
	s.mu.Unlock()
}

func (s *Server) incRecvCounters(n int) {
	s.mu.Lock()
	s.llmRespTotal++
	s.llmRespBytesTotal += int64(n)
	s.mu.Unlock()
}

func (s *Server) logLLMStats() {
	s.mu.RLock()
	avgSent := int64(0)
	if s.llmReqTotal > 0 {
		avgSent = s.llmSentBytesTotal / s.llmReqTotal
	}
	avgRecv := int64(0)
	if s.llmRespTotal > 0 {
		avgRecv = s.llmRespBytesTotal / s.llmRespTotal
	}
	reqs, sentTot, recvTot := s.llmReqTotal, s.llmSentBytesTotal, s.llmRespBytesTotal
	s.mu.RUnlock()
	mins := time.Since(s.startTime).Minutes()
	if mins <= 0 {
		mins = 0.001
	}
	rpm := float64(reqs) / mins
	sentPerMin := float64(sentTot) / mins
	recvPerMin := float64(recvTot) / mins
	logging.Logf("lsp ", "llm stats reqs=%d avg_sent=%d avg_recv=%d sent_total=%d recv_total=%d rpm=%.2f sent_per_min=%.0f recv_per_min=%.0f", reqs, avgSent, avgRecv, sentTot, recvTot, rpm, sentPerMin, recvPerMin)
}

// collectPromptRemovalEdits returns edits to remove all inline prompt markers.
// Supported form (inclusive):
//   - ";...;" where there is no space immediately after the first ';'
//     and no space immediately before the last ';'. An optional single space
//     after the trailing ';' is also removed for cleanliness.
//
// Multiple markers per line are supported.
func (s *Server) collectPromptRemovalEdits(uri string) []TextEdit {
	d := s.getDocument(uri)
	if d == nil || len(d.lines) == 0 {
		return nil
	}
	var edits []TextEdit
	for i, line := range d.lines {
		edits = append(edits, promptRemovalEditsForLine(line, i)...)
	}
	return edits
}

func promptRemovalEditsForLine(line string, lineNum int) []TextEdit {
	if hasDoubleSemicolonTrigger(line) {
		return []TextEdit{{Range: Range{Start: Position{Line: lineNum, Character: 0}, End: Position{Line: lineNum, Character: len(line)}}, NewText: ""}}
	}
	return collectSemicolonMarkers(line, lineNum)
}

func hasDoubleSemicolonTrigger(line string) bool {
	pos := 0
	for pos < len(line) {
		j := strings.Index(line[pos:], ";;")
		if j < 0 {
			return false
		}
		j += pos
		if j+2 < len(line) && line[j+2] != ' ' {
			if k := strings.Index(line[j+2:], ";"); k >= 0 {
				closeIdx := j + 2 + k
				if closeIdx-1 >= 0 && line[closeIdx-1] != ' ' {
					return true
				}
				pos = closeIdx + 1
				continue
			}
			return false
		}
		pos = j + 2
	}
	return false
}

func collectSemicolonMarkers(line string, lineNum int) []TextEdit {
	var edits []TextEdit
	startSemi := 0
	for startSemi < len(line) {
		j := strings.Index(line[startSemi:], ";")
		if j < 0 {
			break
		}
		j += startSemi
		k := strings.Index(line[j+1:], ";")
		if k < 0 {
			break
		}
		if j+1 >= len(line) || line[j+1] == ' ' {
			startSemi = j + 1
			continue
		}
		if line[j+1] == ';' {
			startSemi = j + 2
			continue
		}
		closeIdx := j + 1 + k
		if closeIdx-1 < 0 || line[closeIdx-1] == ' ' {
			startSemi = closeIdx + 1
			continue
		}
		if closeIdx-(j+1) < 1 {
			startSemi = closeIdx + 1
			continue
		}
		endChar := closeIdx + 1
		if endChar < len(line) && line[endChar] == ' ' {
			endChar++
		}
		edits = append(edits, TextEdit{Range: Range{Start: Position{Line: lineNum, Character: j}, End: Position{Line: lineNum, Character: endChar}}, NewText: ""})
		startSemi = endChar
	}
	return edits
}

func inParamList(current string, cursor int) bool {
	if !strings.Contains(current, "func ") {
		return false
	}
	open := strings.Index(current, "(")
	close := strings.Index(current, ")")
	return open >= 0 && cursor > open && (close == -1 || cursor <= close)
}

func buildPrompts(inParams bool, p CompletionParams, above, current, below, funcCtx string) (string, string) {
	if inParams {
		sys := "You are a code completion engine for function signatures. Return only the parameter list contents (without parentheses), no braces, no prose. Prefer idiomatic names and types."
		user := fmt.Sprintf("Cursor is inside the function parameter list. Suggest only the parameter list (no parentheses).\nFunction line: %s\nCurrent line (cursor at %d): %s", funcCtx, p.Position.Character, current)
		return sys, user
	}
	sys := "You are a terse code completion engine. Return only the code to insert, no surrounding prose or backticks. Only continue from the cursor; never repeat characters already present to the left of the cursor on the current line (e.g., if 'name :=' is already typed, only return the right-hand side expression)."
	user := fmt.Sprintf("Provide the next likely code to insert at the cursor.\nFile: %s\nFunction/context: %s\nAbove line: %s\nCurrent line (cursor at character %d): %s\nBelow line: %s\nOnly return the completion snippet.", p.TextDocument.URI, funcCtx, above, p.Position.Character, current, below)
	return sys, user
}

func computeTextEditAndFilter(cleaned string, inParams bool, current string, p CompletionParams) (*TextEdit, string) {
	if inParams {
		open := strings.Index(current, "(")
		close := strings.Index(current, ")")
		if open >= 0 {
			left := open + 1
			right := len(current)
			if close >= 0 && close >= left {
				right = close
			}
			if p.Position.Character < right {
				right = p.Position.Character
			}
			te := &TextEdit{Range: Range{Start: Position{Line: p.Position.Line, Character: left}, End: Position{Line: p.Position.Line, Character: right}}, NewText: cleaned}
			var filter string
			if left >= 0 && right >= left && right <= len(current) {
				filter = strings.TrimLeft(current[left:right], " \t")
			}
			return te, filter
		}
	}
	startChar := computeWordStart(current, p.Position.Character)
	te := &TextEdit{Range: Range{Start: Position{Line: p.Position.Line, Character: startChar}, End: Position{Line: p.Position.Line, Character: p.Position.Character}}, NewText: cleaned}
	filter := strings.TrimLeft(current[startChar:p.Position.Character], " \t")
	return te, filter
}

func computeWordStart(current string, at int) int {
	if at > len(current) {
		at = len(current)
	}
	for at > 0 {
		ch := current[at-1]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			at--
			continue
		}
		break
	}
	return at
}

func isIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

// stripDuplicateAssignmentPrefix removes a duplicated assignment prefix (e.g.,
// "name :=") from the beginning of the model suggestion when that same prefix
// already appears immediately to the left of the cursor on the current line.
// Also handles simple '=' assignments.
func stripDuplicateAssignmentPrefix(prefixBeforeCursor, suggestion string) string {
	s2 := strings.TrimLeft(suggestion, " \t")
	// Prefer := if present at end of prefix
	if idx := strings.LastIndex(prefixBeforeCursor, ":="); idx >= 0 && idx+2 <= len(prefixBeforeCursor) {
		// Ensure only spaces follow in prefix (cursor at end of prefix segment)
		tail := prefixBeforeCursor[idx+2:]
		if strings.TrimSpace(tail) == "" {
			// Move left to include identifier and spaces
			start := idx - 1
			for start >= 0 && (isIdentChar(prefixBeforeCursor[start]) || prefixBeforeCursor[start] == ' ' || prefixBeforeCursor[start] == '\t') {
				start--
			}
			start++
			seg := strings.TrimRight(prefixBeforeCursor[start:idx+2], " \t")
			if strings.HasPrefix(s2, seg) {
				return strings.TrimLeft(s2[len(seg):], " \t")
			}
		}
	}
	// Fallback to plain '=' if present
	if idx := strings.LastIndex(prefixBeforeCursor, "="); idx >= 0 {
		if !(idx > 0 && prefixBeforeCursor[idx-1] == ':') { // not := (handled above)
			tail := prefixBeforeCursor[idx+1:]
			if strings.TrimSpace(tail) == "" {
				start := idx - 1
				for start >= 0 && (isIdentChar(prefixBeforeCursor[start]) || prefixBeforeCursor[start] == ' ' || prefixBeforeCursor[start] == '\t') {
					start--
				}
				start++
				seg := strings.TrimRight(prefixBeforeCursor[start:idx+1], " \t")
				if strings.HasPrefix(s2, seg) {
					return strings.TrimLeft(s2[len(seg):], " \t")
				}
			}
		}
	}
	return suggestion
}

// stripCodeFences removes surrounding Markdown code fences from a model
// response when the entire output is wrapped, e.g. starting with "```go" or
// "```" and ending with "```". It returns the inner content unchanged.
func stripCodeFences(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return t
	}
	lines := splitLines(t)
	// find first and last non-empty lines
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines) - 1
	for end >= 0 && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	if start >= len(lines) || end < 0 || start > end {
		return t
	}
	first := strings.TrimSpace(lines[start])
	last := strings.TrimSpace(lines[end])
	if strings.HasPrefix(first, "```") && last == "```" && end > start {
		inner := strings.Join(lines[start+1:end], "\n")
		return inner
	}
	return t
}

func labelForCompletion(cleaned, filter string) string {
	label := trimLen(firstLine(cleaned))
	if filter != "" && !strings.HasPrefix(strings.ToLower(label), strings.ToLower(filter)) {
		return filter
	}
	return label
}

func (s *Server) fallbackCompletionItems(docStr string) []CompletionItem {
	return []CompletionItem{{
		Label:         "hexai-complete",
		Kind:          1,
		Detail:        "dummy completion",
		InsertText:    "hexai",
		SortText:      "9999",
		Documentation: docStr,
	}}
}
