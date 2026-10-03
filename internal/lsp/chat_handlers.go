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

	"github.com/snonux/hexai/internal/llm"
	"github.com/snonux/hexai/internal/logging"
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
	counts := c.promptCounts(uri, d.lines)
	c.prunePrompts(uri, counts)
	suffix, prefixes, _ := s.chatConfig()
	for i, raw := range d.lines {
		if c.maybeRunInlinePrompt(uri, i, raw, counts) {
			continue
		}
		match, ok := parseChatPromptLine(raw, suffix, prefixes)
		if !ok {
			continue
		}
		if hasChatResponseBelow(d, i) {
			continue
		}
		if !c.handleChatPrompt(uri, i, raw, match, counts) {
			continue // already being answered
		}
		// Only handle one per change tick to avoid flooding
		break
	}
}

type chatPromptLine struct {
	lastNonSpace int
	removeCount  int
	prompt       string
}

// maybeRunInlinePrompt starts the inline prompt on raw, if any, in the
// background. It reports whether raw holds an inline prompt (and so is not
// to be parsed as a chat prompt); counts is as returned by promptCounts.
func (c *chatService) maybeRunInlinePrompt(uri string, lineIdx int, raw string, counts map[string]int) bool {
	s := c.srv
	tag, ok := s.findInlineTag(raw)
	if !ok {
		return false
	}
	if !s.hasLLMTarget(surfaceCompletion) {
		return true
	}
	// Key by the prompt tag rather than the whole line, so text typed after
	// the closing marker while the model works does not start new requests.
	key := inlineKey(uri, tag.text)
	if !c.tryBeginPrompt(key, counts[key]) {
		return true
	}
	s.inflight.Add(1)
	go func() {
		defer s.inflight.Done()
		c.runInlinePrompt(uri, lineIdx, tag.text)
		c.finishPrompt(key)
	}()
	return true
}

func parseChatPromptLine(raw string, suffix string, prefixes []string) (chatPromptLine, bool) {
	if suffix == "" {
		return chatPromptLine{}, false
	}
	if strings.HasPrefix(strings.TrimSpace(raw), ">") {
		// Reply lines inserted by Hexai are never prompts, even when the
		// answer text happens to end in a trigger such as "?>".
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
	if !isSlashCommand(prompt) && !hasTriggerPrefix(raw, last, prefixes) {
		return chatPromptLine{}, false
	}
	return chatPromptLine{lastNonSpace: last, removeCount: removeCount, prompt: prompt}, true
}

// isSlashCommand reports whether prompt looks like a chat slash command such as
// "/reload". A line comment ("// question") is not a command.
func isSlashCommand(prompt string) bool {
	if len(prompt) < 2 || prompt[0] != '/' {
		return false
	}
	ch := prompt[1]
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
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

// handleChatPrompt answers the chat prompt on lineIdx, either directly for a
// slash command or by asking the LLM in the background. It returns false when
// the same prompt is already being answered. counts is as returned by
// promptCounts.
func (c *chatService) handleChatPrompt(uri string, lineIdx int, raw string, match chatPromptLine, counts map[string]int) bool {
	s := c.srv
	key := promptKey(uri, raw)
	if !c.tryBeginPrompt(key, counts[key]) {
		return false
	}
	if resp, ok := c.chatCommandResponse(uri, lineIdx, match.prompt); ok {
		if msg := strings.TrimSpace(resp.message); msg != "" {
			c.applyChatEdits(uri, lineIdx, raw, quoteReply(msg))
		}
		c.finishPrompt(key)
		return true
	}
	s.inflight.Add(1)
	go func() {
		defer s.inflight.Done()
		c.requestChatResponse(uri, lineIdx, raw, match)
		c.finishPrompt(key)
	}()
	return true
}

// requestChatResponse asks the LLM to answer the prompt and inserts the reply.
// It reports whether an edit was sent to the editor.
func (c *chatService) requestChatResponse(uri string, lineIdx int, raw string, match chatPromptLine) bool {
	s := c.srv
	if !s.hasLLMTarget(surfaceChat) {
		return false
	}
	ctx, cancel := s.requestTimeoutContext(25 * time.Second)
	defer cancel()
	pos := Position{Line: lineIdx, Character: byteOffsetToUTF16(raw, match.lastNonSpace+1)}
	msgs := c.buildChatMessages(uri, pos, match.prompt)
	spec := s.buildRequestSpec(surfaceChat)
	logging.Logf("lsp ", "chat llm=requesting model=%s", spec.effectiveModel(""))
	text, err := s.chatWithStats(ctx, surfaceChat, spec, msgs)
	if err != nil {
		logging.Logf("lsp ", "chat llm error: %v", err)
		s.notifyLLMFailure("chat", err)
		return false
	}
	out := strings.TrimSpace(stripCodeFences(text))
	if out == "" {
		return false
	}
	return c.applyChatEdits(uri, lineIdx, raw, quoteReply(out))
}

// applyChatEdits removes the triggering punctuation at end of the prompt line
// and inserts two newlines followed by a new line with the response prefixed.
// It reports whether an edit was sent to the editor.
//
// The response is produced asynchronously, so the document may have changed
// during the LLM round-trip. The prompt line is therefore located again by its
// text (see findPromptLine) and the trigger position recomputed from the live
// line; when the prompt line is gone or no longer ends in a trigger the edit is
// skipped rather than risk deleting user content.
func (c *chatService) applyChatEdits(uri string, lineIdx int, raw string, response string) bool {
	s := c.srv
	d := s.getDocument(uri)
	if d == nil {
		return false
	}
	idx := findPromptLine(d.lines, lineIdx, raw)
	if idx < 0 {
		logging.Logf("lsp ", "chat skip stale edit: prompt line %d no longer present", lineIdx)
		return false
	}
	line := d.lines[idx]
	suffix, prefixes, _ := s.chatConfig()
	match, ok := parseChatPromptLine(line, suffix, prefixes)
	if !ok && raw != "" && strings.HasPrefix(line, strings.TrimRight(raw, " \t")) {
		// Text was typed after the trigger meanwhile; the trigger is still
		// where it was in raw.
		match, ok = parseChatPromptLine(raw, suffix, prefixes)
	}
	if !ok {
		logging.Logf("lsp ", "chat skip stale edit: trigger no longer present on line %d (%q)", idx, line)
		return false
	}
	// 1) Delete the trailing trigger character.
	delStart := Position{Line: idx, Character: byteOffsetToUTF16(line, match.lastNonSpace+1-match.removeCount)}
	delEnd := Position{Line: idx, Character: byteOffsetToUTF16(line, match.lastNonSpace+1)}
	// 2) Insert two newlines and the response at end-of-line, then one extra blank line
	insPos := Position{Line: idx, Character: byteOffsetToUTF16(line, len(line))}
	insert := "\n\n" + strings.TrimRight(response, "\n") + "\n\n"
	edits := []TextEdit{
		{Range: Range{Start: delStart, End: delEnd}, NewText: ""},
		{Range: Range{Start: insPos, End: insPos}, NewText: insert},
	}
	we := WorkspaceEdit{Changes: map[string][]TextEdit{uri: edits}}
	s.clientApplyEdit("Hexai: insert chat response", we)
	return true
}

// quoteReply prefixes every line of a chat reply with "> " (">" for blank
// lines), so multi-line answers are recognized as replies by the duplicate
// check and the chat history, and are never mistaken for new prompts.
func quoteReply(text string) string {
	lines := splitLines(strings.TrimRight(text, "\n"))
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			lines[i] = ">"
			continue
		}
		lines[i] = "> " + ln
	}
	return strings.Join(lines, "\n")
}

// findPromptLine returns the index of the line whose text equals raw,
// preferring lineIdx and otherwise the nearest match (lines above the prompt
// may have been added or removed meanwhile). Failing that it looks for a line
// that still starts with raw (the user kept typing after the trigger). When
// nothing matches, the prompt line itself was edited and -1 is returned: the
// answer belongs to the old question, and the edited line, if still a prompt,
// is asked separately.
func findPromptLine(lines []string, lineIdx int, raw string) int {
	if idx := nearestLine(lines, lineIdx, func(ln string) bool { return ln == raw }); idx >= 0 {
		return idx
	}
	if trimmed := strings.TrimRight(raw, " \t"); trimmed != "" {
		return nearestLine(lines, lineIdx, func(ln string) bool { return strings.HasPrefix(ln, trimmed) })
	}
	return -1
}

// nearestLine returns the index of the line closest to lineIdx for which
// match is true, or -1.
func nearestLine(lines []string, lineIdx int, match func(string) bool) int {
	for dist := 0; dist < len(lines); dist++ {
		up, down := lineIdx-dist, lineIdx+dist
		if up >= 0 && up < len(lines) && match(lines[up]) {
			return up
		}
		if down >= 0 && down < len(lines) && match(lines[down]) {
			return down
		}
		if up < 0 && down >= len(lines) {
			break
		}
	}
	return -1
}

// runInlinePrompt completes the inline prompt tagText found near line lineIdx
// and applies the result. It reports whether an edit was sent.
func (c *chatService) runInlinePrompt(uri string, lineIdx int, tagText string) bool {
	s := c.srv
	d := s.getDocument(uri)
	if d == nil || !s.hasLLMTarget(surfaceCompletion) {
		return false
	}
	idx, tag := s.findInlineTagLine(d.lines, lineIdx, tagText)
	if idx < 0 {
		return false // the prompt was edited before we got to it
	}
	// The cursor sits right after the closing marker, so text typed after
	// the prompt is neither taken as the typed prefix nor replaced.
	line := d.lines[idx]
	p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Line: idx, Character: byteOffsetToUTF16(line, tag.end)}}
	p.Context = map[string]int{"triggerKind": 1}
	above, current, below, funcCtx := s.lineContext(uri, p.Position)
	docStr := s.completion.buildDocString(p, above, current, below, funcCtx)
	newFunc := s.isDefiningNewFunction(uri, p.Position)
	extra, hasExtra := s.buildAdditionalContext(newFunc, uri, p.Position)
	items, ok, _ := s.completion.tryLLMCompletion(p, above, current, below, funcCtx, docStr, hasExtra, extra)
	if !ok || len(items) == 0 || items[0].TextEdit == nil {
		s.notifyLLMFailure("inline prompt", nil)
		return false
	}
	return c.applyInlineCompletion(uri, idx, tagText, items[0].TextEdit.NewText)
}

// applyInlineCompletion replaces the inline prompt tagText with text (or, for
// the line-replacing ">>!" form, the whole line). Like applyChatEdits it
// re-locates the prompt in the live document, since the LLM call may have
// taken a while, and skips the edit when the prompt is gone.
func (c *chatService) applyInlineCompletion(uri string, lineIdx int, tagText string, text string) bool {
	s := c.srv
	d := s.getDocument(uri)
	if d == nil || strings.TrimSpace(text) == "" {
		return false
	}
	idx, tag := s.findInlineTagLine(d.lines, lineIdx, tagText)
	if idx < 0 {
		logging.Logf("lsp ", "inline prompt skip stale edit: prompt %q no longer present", tagText)
		return false
	}
	line := d.lines[idx]
	start, end := tag.start, tag.end
	if tag.wholeLine {
		start, end = 0, len(line)
	}
	rng := Range{
		Start: Position{Line: idx, Character: byteOffsetToUTF16(line, start)},
		End:   Position{Line: idx, Character: byteOffsetToUTF16(line, end)},
	}
	we := WorkspaceEdit{Changes: map[string][]TextEdit{uri: {{Range: rng, NewText: text}}}}
	s.clientApplyEdit("Hexai: inline prompt", we)
	return true
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
	// Clamp the starting index to the current document bounds. lineIdx is derived
	// from a position captured when the chat prompt was detected, but the chat
	// response is applied asynchronously (see applyChatEdits): a concurrent
	// didChange may have shrunk the document so that lineIdx-1 now exceeds
	// len(d.lines)-1. Without clamping, the d.lines[i] accesses below would panic
	// with index-out-of-range. We start from the last valid line instead.
	i := lineIdx - 1
	if i >= len(d.lines) {
		i = len(d.lines) - 1
	}
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
