// Summary: Document open/change/close and in-editor chat handlers split out of handlers.go.
package lsp

import (
	"context"
	"encoding/json"
	"hexai/internal/llm"
	"hexai/internal/logging"
	"strings"
	"time"
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
	if s.llmClient == nil {
		return
	}
	d := s.getDocument(uri)
	if d == nil || len(d.lines) == 0 {
		return
	}
	for i, raw := range d.lines {
		// Find last non-space character index
		j := len(raw) - 1
		for j >= 0 {
			if raw[j] == ' ' || raw[j] == '\t' {
				j--
				continue
			}
			break
		}
		if j < 1 {
			continue
		} // need at least two chars
		pair := raw[j-1 : j+1]
		isTrigger := pair == "?>" || pair == "!>" || pair == ":>" || pair == ";>"
		if !isTrigger {
			continue
		}
		// Avoid double-answering: if the next non-empty line starts with '>' we skip.
		k := i + 1
		for k < len(d.lines) && strings.TrimSpace(d.lines[k]) == "" {
			k++
		}
		if k < len(d.lines) && strings.HasPrefix(strings.TrimSpace(d.lines[k]), ">") {
			continue
		}
		// Derive prompt by removing only the trailing '>'
		removeCount := 1
		base := raw[:j+1-removeCount]
		prompt := strings.TrimSpace(base)
		if prompt == "" {
			continue
		}
		lineIdx := i
		lastIdx := j
		go func(prompt string, remove int) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			sys := "You are a helpful coding assistant. Answer concisely and clearly."
			// Build short conversation history from the document above this line
			history := s.buildChatHistory(uri, lineIdx, prompt)
			msgs := append([]llm.Message{{Role: "system", Content: sys}}, history...)
			opts := s.llmRequestOpts()
			logging.Logf("lsp ", "chat llm=requesting model=%s", s.llmClient.DefaultModel())
			text, err := s.llmClient.Chat(ctx, msgs, opts...)
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
		q = stripTrailingTrigger(q)
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
func stripTrailingTrigger(sx string) string {
	s := strings.TrimRight(sx, " \t")
	if len(s) >= 2 && s[len(s)-1] == '>' { // new triggers
		prev := s[len(s)-2]
		if prev == '?' || prev == '!' || prev == ':' || prev == ';' {
			return strings.TrimRight(s[:len(s)-1], " \t")
		}
	}
	if strings.HasSuffix(s, ";;") { // legacy inline cleanup used in history building
		return strings.TrimRight(strings.TrimSuffix(s, ";;"), " \t")
	}
	if len(s) == 0 {
		return sx
	}
	last := s[len(s)-1]
	switch last { // legacy: remove one trailing punctuation
	case '?', '!', ':':
		return strings.TrimRight(s[:len(s)-1], " \t")
	default:
		return sx
	}
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
