// In-editor chat handling for the LSP server. These are methods on chatService
// (the extracted in-editor chat subsystem), which detects chat/inline-prompt
// trigger lines, builds the rolling transcript history and request messages,
// and applies the LLM reply back into the document. It reaches into Server (via
// c.srv, aliased to s) for shared infrastructure such as LLM clients, document
// access, config and edit dispatch.
package lsp

import (
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/logging"
)

// --- in-editor chat (";C ...") ---

// detectAndHandleChat scans the current document for any line that starts with
// a new trigger pair (e.g., "?>" ",>" ":>" ";>") at EOL and inserts the LLM
// reply below.
func (c *chatService) detectAndHandleChat(uri string) {
	s := c.srv
	d := s.getDocument(uri)
	if d == nil || len(d.lines) == 0 {
		return
	}
	suffix, prefixes, _ := s.chatConfig()
	openStr, _, openChar, closeChar := s.inlineMarkers()
	for i, raw := range d.lines {
		if c.maybeRunInlinePrompt(uri, i, raw, openStr, openChar, closeChar) {
			continue
		}
		match, ok := parseChatPromptLine(raw, suffix, prefixes)
		if !ok {
			continue
		}
		if hasChatResponseBelow(d, i) {
			continue
		}
		c.handleChatPrompt(uri, i, match)
		// Only handle one per change tick to avoid flooding
		break
	}
}

type chatPromptLine struct {
	lastNonSpace int
	removeCount  int
	prompt       string
}

func (c *chatService) maybeRunInlinePrompt(uri string, lineIdx int, raw string, openStr string, openChar byte, closeChar byte) bool {
	s := c.srv
	if !lineHasInlinePrompt(raw, openStr, openChar, closeChar) {
		return false
	}
	if s.currentLLMClient() != nil {
		pos := Position{Line: lineIdx, Character: len(raw)}
		s.inflight.Add(1)
		go func() {
			defer s.inflight.Done()
			c.runInlinePrompt(uri, pos)
		}()
	}
	return true
}

func parseChatPromptLine(raw string, suffix string, prefixes []string) (chatPromptLine, bool) {
	if suffix == "" {
		return chatPromptLine{}, false
	}
	last := findLastNonSpaceIndex(raw)
	if last < 0 || string(raw[last]) != suffix {
		return chatPromptLine{}, false
	}
	removeCount := len(suffix)
	baseEnd := last + 1 - removeCount
	if baseEnd < 0 {
		return chatPromptLine{}, false
	}
	prompt := strings.TrimSpace(raw[:baseEnd])
	if prompt == "" {
		return chatPromptLine{}, false
	}
	if !strings.HasPrefix(prompt, "/") && !hasTriggerPrefix(raw, last, prefixes) {
		return chatPromptLine{}, false
	}
	return chatPromptLine{lastNonSpace: last, removeCount: removeCount, prompt: prompt}, true
}

func findLastNonSpaceIndex(raw string) int {
	for i := len(raw) - 1; i >= 0; i-- {
		if raw[i] != ' ' && raw[i] != '\t' {
			return i
		}
	}
	return -1
}

func hasTriggerPrefix(raw string, suffixIdx int, prefixes []string) bool {
	if suffixIdx < 1 {
		return false
	}
	prev := string(raw[suffixIdx-1])
	for _, pfx := range prefixes {
		if prev == pfx {
			return true
		}
	}
	return false
}

func hasChatResponseBelow(d *document, lineIdx int) bool {
	for i := lineIdx + 1; i < len(d.lines); i++ {
		trimmed := strings.TrimSpace(d.lines[i])
		if trimmed == "" {
			continue
		}
		return strings.HasPrefix(trimmed, ">")
	}
	return false
}

func (c *chatService) handleChatPrompt(uri string, lineIdx int, match chatPromptLine) {
	s := c.srv
	if resp, ok := c.chatCommandResponse(uri, lineIdx, match.prompt); ok {
		msg := strings.TrimSpace(resp.message)
		if msg != "" {
			c.applyChatEdits(uri, lineIdx, match.lastNonSpace, match.removeCount, "> "+msg)
		}
		return
	}
	s.inflight.Add(1)
	go func() {
		defer s.inflight.Done()
		c.requestChatResponse(uri, lineIdx, match)
	}()
}

func (c *chatService) requestChatResponse(uri string, lineIdx int, match chatPromptLine) {
	s := c.srv
	ctx, cancel := s.requestTimeoutContext(25 * time.Second)
	defer cancel()
	pos := Position{Line: lineIdx, Character: match.lastNonSpace + 1}
	msgs := c.buildChatMessages(uri, pos, match.prompt)
	spec := s.buildRequestSpec(surfaceChat)
	client := s.clientFor(spec)
	if client == nil {
		return
	}
	modelUsed := spec.effectiveModel(client.DefaultModel())
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
	c.applyChatEdits(uri, lineIdx, match.lastNonSpace, match.removeCount, "> "+out)
}

// applyChatEdits removes the triggering punctuation at end of the line and
// inserts two newlines followed by a new line with the response prefixed.
func (c *chatService) applyChatEdits(uri string, lineIdx int, lastNonSpace int, removeCount int, response string) {
	s := c.srv
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

func (c *chatService) runInlinePrompt(uri string, pos Position) {
	s := c.srv
	if s.currentLLMClient() == nil {
		return
	}
	d := s.getDocument(uri)
	if d == nil || pos.Line < 0 || pos.Line >= len(d.lines) {
		return
	}
	line := d.lines[pos.Line]
	openStr, _, openChar, closeChar := s.inlineMarkers()
	if !lineHasInlinePrompt(line, openStr, openChar, closeChar) {
		return
	}
	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Line: pos.Line, Character: len(line)}}
	p.Context = map[string]int{"triggerKind": 1}
	above, current, below, funcCtx := s.lineContext(uri, p.Position)
	docStr := s.completion.buildDocString(p, above, current, below, funcCtx)
	newFunc := s.isDefiningNewFunction(uri, p.Position)
	extra, hasExtra := s.buildAdditionalContext(newFunc, uri, p.Position)
	items, ok, _ := s.completion.tryLLMCompletion(p, above, current, below, funcCtx, docStr, hasExtra, extra)
	if !ok || len(items) == 0 {
		return
	}
	c.applyInlineCompletion(uri, items[0])
}

func (c *chatService) applyInlineCompletion(uri string, item CompletionItem) {
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
	c.srv.clientApplyEdit("Hexai: inline prompt", we)
}

// buildChatHistory walks upwards from the current line to collect the most recent
// Q/A pairs in the in-editor transcript. Returns messages ending with current prompt.
func (c *chatService) buildChatHistory(uri string, lineIdx int, currentPrompt string) []llm.Message {
	s := c.srv
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
		q = c.stripTrailingTrigger(q)
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
func (c *chatService) stripTrailingTrigger(sx string) string {
	trim := strings.TrimRight(sx, " \t")
	if len(trim) == 0 {
		return sx
	}
	_, prefixes, suffixChar := c.srv.chatConfig()
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
func (c *chatService) buildChatMessages(uri string, pos Position, prompt string) []llm.Message {
	s := c.srv
	// Base system and history
	cfg := s.currentConfig()
	sys := cfg.PromptChatSystem
	// Determine line index for history from position
	lineIdx := pos.Line
	history := c.buildChatHistory(uri, lineIdx, prompt)
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
