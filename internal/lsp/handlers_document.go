// Summary: Document open/change/close and in-editor chat handlers split out of handlers.go.
package lsp

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/logging"
)

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
		// Detect in-editor chat trigger lines and respond inline.
		s.detectAndHandleChat(p.TextDocument.URI)
	}
}

func (s *Server) handleDidClose(req Request) {
	var p DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params, &p); err == nil {
		s.deleteDocument(p.TextDocument.URI)
		s.markActivity()
	}
}

// docBeforeAfter returns the full document text split at the given position.
// The returned strings are the text before the cursor (inclusive of anything
// left of the position) and the text after the cursor.
func (s *Server) docBeforeAfter(uri string, pos Position) (string, string) {
	d := s.getDocument(uri)
	if d == nil {
		return "", ""
	}
	// Clamp indices
	line := pos.Line
	if line < 0 {
		line = 0
	}
	if line >= len(d.lines) {
		line = len(d.lines) - 1
	}
	col := pos.Character
	if col < 0 {
		col = 0
	}
	if col > len(d.lines[line]) {
		col = len(d.lines[line])
	}
	// Build before
	var b strings.Builder
	for i := 0; i < line; i++ {
		b.WriteString(d.lines[i])
		b.WriteByte('\n')
	}
	b.WriteString(d.lines[line][:col])
	before := b.String()
	// Build after
	var a strings.Builder
	a.WriteString(d.lines[line][col:])
	for i := line + 1; i < len(d.lines); i++ {
		a.WriteByte('\n')
		a.WriteString(d.lines[i])
	}
	return before, a.String()
}

// --- in-editor chat (";C ...") ---

// detectAndHandleChat scans the current document for any line that starts with
// a new trigger pair (e.g., "?>" ",>" ":>" ";>") at EOL and inserts the LLM
// reply below.
func (s *Server) detectAndHandleChat(uri string) {
	d := s.getDocument(uri)
	if d == nil || len(d.lines) == 0 {
		return
	}
	suffix, prefixes, _ := s.chatConfig()
	_, _, openChar, closeChar := s.inlineMarkers()
	for i, raw := range d.lines {
		if lineHasInlinePrompt(raw, openChar, closeChar) {
			if s.currentLLMClient() != nil {
				pos := Position{Line: i, Character: len(raw)}
				go s.runInlinePrompt(uri, pos)
			}
			continue
		}
		// Find last non-space character index
		j := len(raw) - 1
		for j >= 0 {
			if raw[j] == ' ' || raw[j] == '\t' {
				j--
				continue
			}
			break
		}
		if j < 0 {
			continue
		}
		// Check suffix and derive the prompt text before validating prefixes
		if suffix == "" {
			continue
		}
		if string(raw[j]) != suffix {
			continue
		}
		removeCount := len(suffix)
		base := raw[:j+1-removeCount]
		prompt := strings.TrimSpace(base)
		if prompt == "" {
			continue
		}
		// Slash commands (`/foo>`) do not require a prefix trigger.
		isCommand := strings.HasPrefix(prompt, "/")
		if !isCommand {
			// Require at least one char before suffix and that char must be in chatPrefixes
			if j < 1 {
				continue
			}
			prev := string(raw[j-1])
			match := false
			for _, pfx := range prefixes {
				if prev == pfx {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		// Avoid double-answering: if the next non-empty line starts with '>' we skip.
		k := i + 1
		for k < len(d.lines) && strings.TrimSpace(d.lines[k]) == "" {
			k++
		}
		if k < len(d.lines) && strings.HasPrefix(strings.TrimSpace(d.lines[k]), ">") {
			continue
		}
		lineIdx := i
		lastIdx := j
		if resp, ok := s.chatCommandResponse(uri, lineIdx, prompt); ok {
			msg := strings.TrimSpace(resp.message)
			if msg != "" {
				s.applyChatEdits(uri, lineIdx, lastIdx, removeCount, "> "+msg)
			}
			return
		}
		go func(prompt string, remove int) {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			// Build messages with history and context_mode aware extras.
			pos := Position{Line: lineIdx, Character: lastIdx + 1}
			msgs := s.buildChatMessages(uri, pos, prompt)
			spec := s.buildRequestSpec(surfaceChat)
			client := s.clientFor(spec)
			if client == nil {
				return
			}
			modelUsed := spec.effectiveModel()
			if strings.TrimSpace(modelUsed) == "" {
				modelUsed = client.DefaultModel()
			}
			logging.Logf("lsp ", "chat llm=requesting model=%s", modelUsed)
			text, err := s.chatWithStats(ctx, surfaceChat, spec, msgs)
			if err != nil {
				logging.Logf("lsp ", "chat llm error: %v", err)
				return
			}
			out := strings.TrimSpace(stripCodeFences(text))
			if out == "" {
				return
			}
			s.applyChatEdits(uri, lineIdx, lastIdx, remove, "> "+out)
		}(prompt, removeCount)
		// Only handle one per change tick to avoid flooding
		break
	}
}

// applyChatEdits removes the triggering punctuation at end of the line and
// inserts two newlines followed by a new line with the response prefixed.
func (s *Server) applyChatEdits(uri string, lineIdx int, lastNonSpace int, removeCount int, response string) {
	d := s.getDocument(uri)
	if d == nil {
		return
	}
	// 1) Delete the trailing punctuation (1 or 2 chars)
	delStart := Position{Line: lineIdx, Character: lastNonSpace + 1 - removeCount}
	delEnd := Position{Line: lineIdx, Character: lastNonSpace + 1}
	// 2) Insert two newlines and the response at end-of-line, then one extra blank line
	insPos := Position{Line: lineIdx, Character: len(d.lines[lineIdx])}
	resp := strings.TrimRight(response, "\n") + "\n"
	insert := "\n\n" + resp + "\n"
	edits := []TextEdit{
		{Range: Range{Start: delStart, End: delEnd}, NewText: ""},
		{Range: Range{Start: insPos, End: insPos}, NewText: insert},
	}
	we := WorkspaceEdit{Changes: map[string][]TextEdit{uri: edits}}
	s.clientApplyEdit("Hexai: insert chat response", we)
}

func (s *Server) runInlinePrompt(uri string, pos Position) {
	if s.currentLLMClient() == nil {
		return
	}
	d := s.getDocument(uri)
	if d == nil || pos.Line < 0 || pos.Line >= len(d.lines) {
		return
	}
	line := d.lines[pos.Line]
	_, _, openChar, closeChar := s.inlineMarkers()
	if !lineHasInlinePrompt(line, openChar, closeChar) {
		return
	}
	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Line: pos.Line, Character: len(line)}}
	p.Context = map[string]int{"triggerKind": 1}
	above, current, below, funcCtx := s.lineContext(uri, p.Position)
	docStr := s.buildDocString(p, above, current, below, funcCtx)
	newFunc := s.isDefiningNewFunction(uri, p.Position)
	extra, hasExtra := s.buildAdditionalContext(newFunc, uri, p.Position)
	items, ok := s.tryLLMCompletion(p, above, current, below, funcCtx, docStr, hasExtra, extra)
	if !ok || len(items) == 0 {
		return
	}
	s.applyInlineCompletion(uri, items[0])
}

func (s *Server) applyInlineCompletion(uri string, item CompletionItem) {
	var edits []TextEdit
	if len(item.AdditionalTextEdits) > 0 {
		edits = append(edits, item.AdditionalTextEdits...)
	}
	if item.TextEdit != nil {
		edits = append(edits, *item.TextEdit)
	}
	if len(edits) == 0 {
		return
	}
	we := WorkspaceEdit{Changes: map[string][]TextEdit{uri: edits}}
	s.clientApplyEdit("Hexai: inline prompt", we)
}

// buildChatHistory walks upwards from the current line to collect the most recent
// Q/A pairs in the in-editor transcript. Returns messages ending with current prompt.
func (s *Server) buildChatHistory(uri string, lineIdx int, currentPrompt string) []llm.Message {
	d := s.getDocument(uri)
	if d == nil {
		return []llm.Message{{Role: "user", Content: currentPrompt}}
	}
	type pair struct{ q, a string }
	pairs := []pair{}
	i := lineIdx - 1
	for i >= 0 && len(pairs) < 3 {
		for i >= 0 && strings.TrimSpace(d.lines[i]) == "" {
			i--
		}
		if i < 0 {
			break
		}
		if !strings.HasPrefix(strings.TrimSpace(d.lines[i]), ">") {
			break
		}
		var replyLines []string
		for i >= 0 {
			line := strings.TrimSpace(d.lines[i])
			if strings.HasPrefix(line, ">") {
				replyLines = append([]string{strings.TrimSpace(strings.TrimPrefix(line, ">"))}, replyLines...)
				i--
				continue
			}
			break
		}
		for i >= 0 && strings.TrimSpace(d.lines[i]) == "" {
			i--
		}
		if i < 0 {
			break
		}
		q := strings.TrimSpace(d.lines[i])
		q = s.stripTrailingTrigger(q)
		pairs = append([]pair{{q: q, a: strings.Join(replyLines, "\n")}}, pairs...)
		i--
	}
	msgs := make([]llm.Message, 0, len(pairs)*2+1)
	for _, p := range pairs {
		if strings.TrimSpace(p.q) != "" {
			msgs = append(msgs, llm.Message{Role: "user", Content: p.q})
		}
		if strings.TrimSpace(p.a) != "" {
			msgs = append(msgs, llm.Message{Role: "assistant", Content: p.a})
		}
	}
	msgs = append(msgs, llm.Message{Role: "user", Content: currentPrompt})
	return msgs
}

// stripTrailingTrigger removes the trailing chat trigger punctuation from a line if present.
func (s *Server) stripTrailingTrigger(sx string) string {
	trim := strings.TrimRight(sx, " \t")
	if len(trim) == 0 {
		return sx
	}
	_, prefixes, suffixChar := s.chatConfig()
	if len(trim) >= 2 && suffixChar != 0 && trim[len(trim)-1] == suffixChar {
		prev := string(trim[len(trim)-2])
		for _, pf := range prefixes {
			if prev == pf {
				return strings.TrimRight(trim[:len(trim)-1], " \t")
			}
		}
	}
	last := trim[len(trim)-1]
	switch last {
	case '?', '!', ':':
		return strings.TrimRight(trim[:len(trim)-1], " \t")
	default:
		return sx
	}
}

// buildChatMessages assembles the chat request messages using:
// - system from prompts.chat.system
// - rolling in-editor history up to current prompt
// - optional extra context per general.context_mode (window/full-file/new-func)
func (s *Server) buildChatMessages(uri string, pos Position, prompt string) []llm.Message {
	// Base system and history
	cfg := s.currentConfig()
	sys := cfg.PromptChatSystem
	// Determine line index for history from position
	lineIdx := pos.Line
	history := s.buildChatHistory(uri, lineIdx, prompt)
	// Start with system
	msgs := []llm.Message{{Role: "system", Content: sys}}
	// Optional additional context like completion path (insert before history so last remains the prompt)
	newFunc := s.isDefiningNewFunction(uri, pos)
	if extra, has := s.buildAdditionalContext(newFunc, uri, pos); has && strings.TrimSpace(extra) != "" {
		// Reuse completion's extra header template to avoid duplication
		header := renderTemplate(cfg.PromptCompletionExtraHeader, map[string]string{"context": extra})
		if strings.TrimSpace(header) == "" {
			header = extra
		}
		msgs = append(msgs, llm.Message{Role: "user", Content: header})
	}
	// Then add history (which ends with the current prompt)
	msgs = append(msgs, history...)
	return msgs
}

// clientApplyEdit sends a workspace/applyEdit request to the client.
func (s *Server) clientApplyEdit(label string, edit WorkspaceEdit) {
	params := ApplyWorkspaceEditParams{Label: label, Edit: edit}
	id := s.nextReqID()
	req := Request{JSONRPC: "2.0", ID: id, Method: "workspace/applyEdit"}
	b, _ := json.Marshal(params)
	req.Params = b
	s.writeMessage(req)
}

// nextReqID returns a unique json.RawMessage id for server-initiated requests.
func (s *Server) nextReqID() json.RawMessage {
	s.mu.Lock()
	s.nextID++
	idNum := s.nextID
	s.mu.Unlock()
	b, _ := json.Marshal(idNum)
	return b
}

// clientShowDocument asks the client to open/focus a document and select a range.
func (s *Server) clientShowDocument(uri string, sel *Range) {
	var params struct {
		URI       string `json:"uri"`
		External  bool   `json:"external,omitempty"`
		TakeFocus bool   `json:"takeFocus,omitempty"`
		Selection *Range `json:"selection,omitempty"`
	}
	params.URI = uri
	params.TakeFocus = true
	params.Selection = sel
	id := s.nextReqID()
	req := Request{JSONRPC: "2.0", ID: id, Method: "window/showDocument"}
	b, _ := json.Marshal(params)
	req.Params = b
	s.writeMessage(req)
}

// deferShowDocument schedules a showDocument after a short delay to allow the client
// time to apply any pending edits (e.g., create the file before focusing it).
func (s *Server) deferShowDocument(uri string, sel Range) {
	go func() {
		time.Sleep(120 * time.Millisecond)
		s.clientShowDocument(uri, &sel)
	}()
}
