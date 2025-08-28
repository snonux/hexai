// Summary: LSP JSON-RPC handlers; implements core methods and integrates with the LLM client when enabled.
// TODO: Split this up into multiple smaller files.
package lsp

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (s *Server) handle(req Request) {
	if h, ok := s.handlers[req.Method]; ok {
		h(req)
		return
	}
	if len(req.ID) != 0 {
		s.reply(req.ID, nil, &RespError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)})
	}
}

// handleInitialize moved to handlers_init.go

// llmRequestOpts moved to handlers_utils.go

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

// diagnosticsInRange parses the CodeAction context and returns diagnostics
// that overlap the given selection range. If the context is missing or does
// not contain diagnostics, returns an empty slice.
// CodeAction-related handlers and helpers moved to handlers_codeaction.go

// extractRangeText moved to handlers_utils.go

// handleInitialized moved to handlers_init.go

// handleShutdown moved to handlers_init.go

// handleExit moved to handlers_init.go

// handleDidOpen moved to handlers_document.go

// handleDidChange moved to handlers_document.go

// handleDidClose moved to handlers_document.go

// handleCompletion moved to handlers_completion.go

func (s *Server) reply(id json.RawMessage, result any, err *RespError) {
	resp := Response{JSONRPC: "2.0", ID: id, Result: result, Error: err}
	s.writeMessage(resp)
}

// docBeforeAfter returns the full document text split at the given position.
// The returned strings are the text before the cursor (inclusive of anything
// left of the position) and the text after the cursor.
// docBeforeAfter moved to handlers_document.go

// extractTriggerInfo returns the LSP completion TriggerKind and TriggerCharacter
// if provided by the client; when absent it returns zeros.
// extractTriggerInfo moved to handlers_completion.go

// --- in-editor chat (";C ...") ---

// detectAndHandleChat scans the current document for any line that starts with
// ";C" and appears to be awaiting a response (i.e., followed by a blank line
// and no non-empty answer line yet). If found, it asks the LLM and inserts the
// answer below the blank line, leaving exactly one empty line between prompt
// and response.
// detectAndHandleChat moved to handlers_document.go

// applyChatEdits removes the triggering punctuation at end of the line and
// inserts two newlines followed by a new line with the response prefixed.
// applyChatEdits moved to handlers_document.go

// buildChatHistory walks upwards from the current line to collect the most recent
// Q/A pairs in the in-editor transcript. It returns messages in chronological order
// ending with the current user prompt. Limits to a small number of pairs to control tokens.
// buildChatHistory moved to handlers_document.go

// stripTrailingTrigger removes a single trailing punctuation from the set
// [?,!,:] or both semicolons if present at end, mirroring the inline trigger rules.
// stripTrailingTrigger moved to handlers_document.go

// clientApplyEdit sends a workspace/applyEdit request to the client.
// clientApplyEdit moved to handlers_document.go

// nextReqID returns a unique json.RawMessage id for server-initiated requests.
// nextReqID moved to handlers_document.go

// --- completion helpers ---

// buildDocString moved to handlers_completion.go

// logCompletionContext moved to handlers_completion.go

// tryLLMCompletion moved to handlers_completion.go

// parseManualInvoke inspects the LSP completion context and reports whether the user manually invoked completion.
// parseManualInvoke moved to handlers_completion.go

// shouldSuppressForChatTriggerEOL returns true when a chat trigger like ">" follows ?, !, :, or ; at EOL.
// shouldSuppressForChatTriggerEOL moved to handlers_completion.go

// prefixHeuristicAllows applies minimal prefix rules unless inlinePrompt or structural triggers apply.
// prefixHeuristicAllows moved to handlers_completion.go

// tryProviderNativeCompletion attempts provider-native completion and returns items when successful.
// tryProviderNativeCompletion moved to handlers_completion.go

// buildCompletionMessages constructs the LLM messages for completion.
// buildCompletionMessages moved to handlers_completion.go

// postProcessCompletion normalizes and deduplicates completion text and applies indentation rules.
// postProcessCompletion moved to handlers_completion.go

// busyCompletionItem builds a visible, non-inserting completion item indicating
// that an LLM request is already in flight.
func (s *Server) busyCompletionItem() CompletionItem {
	prov := ""
	model := ""
	if s.llmClient != nil {
		prov = s.llmClient.Name()
		model = s.llmClient.DefaultModel()
	}
	label := "Hexai: LLM busy"
	if prov != "" && model != "" {
		label += " (" + prov + ":" + model + ")"
	}
	return CompletionItem{
		Label:         label,
		Detail:        "Another request is running; only one is allowed concurrently",
		InsertText:    "",
		FilterText:    "",
		SortText:      "~~~~~busy", // float to top
		Documentation: "Hexai is processing a previous request. Please retry shortly.",
	}
}

func (s *Server) isLLMBusy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.llmBusy
}

func (s *Server) setLLMBusy(v bool) {
	s.mu.Lock()
	s.llmBusy = v
	s.mu.Unlock()
}

// --- small completion cache (last ~10 entries) ---

func (s *Server) completionCacheKey(p CompletionParams, above, current, below, funcCtx string, inParams bool, hasExtra bool, extraText string) string {
	// Normalize left-of-cursor by trimming trailing spaces/tabs
	idx := p.Position.Character
	if idx > len(current) {
		idx = len(current)
	}
	left := strings.TrimRight(current[:idx], " \t")
	right := ""
	if idx < len(current) {
		right = current[idx:]
	}
	prov := ""
	model := ""
	if s.llmClient != nil {
		prov = s.llmClient.Name()
		model = s.llmClient.DefaultModel()
	}
	temp := ""
	if s.codingTemperature != nil {
		temp = fmt.Sprintf("%.3f", *s.codingTemperature)
	}
	extra := ""
	if hasExtra {
		extra = strings.TrimSpace(extraText)
	}
	// Compose a key from essential context parts
	return strings.Join([]string{
		"v1", // version for future-proofing
		prov,
		model,
		temp,
		p.TextDocument.URI,
		fmt.Sprintf("%d:%d", p.Position.Line, len(left)),
		above,
		left,
		right,
		below,
		funcCtx,
		fmt.Sprintf("params=%t", inParams),
		extra,
	}, "\x1f") // use unit separator to avoid collisions
}

func (s *Server) completionCacheGet(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.compCache[key]
	if !ok {
		return "", false
	}
	// move to most-recent
	s.compCacheTouchLocked(key)
	return v, true
}

func (s *Server) completionCachePut(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.compCache == nil {
		s.compCache = make(map[string]string)
	}
	if _, exists := s.compCache[key]; !exists {
		s.compCacheOrder = append(s.compCacheOrder, key)
		s.compCache[key] = value
		if len(s.compCacheOrder) > 10 {
			// evict oldest
			old := s.compCacheOrder[0]
			s.compCacheOrder = s.compCacheOrder[1:]
			delete(s.compCache, old)
		}
		return
	}
	// update existing and mark most-recent
	s.compCache[key] = value
	s.compCacheTouchLocked(key)
}

func (s *Server) compCacheTouchLocked(key string) {
	// assumes s.mu is held
	// remove any existing occurrence of key in order slice
	idx := -1
	for i, k := range s.compCacheOrder {
		if k == key {
			idx = i
			break
		}
	}
	if idx >= 0 {
		s.compCacheOrder = append(append([]string{}, s.compCacheOrder[:idx]...), s.compCacheOrder[idx+1:]...)
	}
	s.compCacheOrder = append(s.compCacheOrder, key)
}

// isTriggerEvent returns true when the completion request appears to be caused
// by typing one of our configured trigger characters. It checks the LSP
// CompletionContext if provided and also falls back to inspecting the character
// immediately to the left of the cursor.
func (s *Server) isTriggerEvent(p CompletionParams, current string) bool {
	// 1) Inspect LSP completion context if present
	if p.Context != nil {
		var ctx struct {
			TriggerKind      int    `json:"triggerKind"`
			TriggerCharacter string `json:"triggerCharacter,omitempty"`
		}
		if raw, ok := p.Context.(json.RawMessage); ok {
			_ = json.Unmarshal(raw, &ctx)
		} else {
			b, _ := json.Marshal(p.Context)
			_ = json.Unmarshal(b, &ctx)
		}
		// If the line contains a bare ';;' (no ';;text;'), do not treat as a trigger source.
		if strings.Contains(current, ";;") && !hasDoubleSemicolonTrigger(current) {
			return false
		}
		// TriggerKind 1 = Invoked (manual) — always allow (unless bare ';;' above)
		if ctx.TriggerKind == 1 {
			return true
		}
		// TriggerKind 2 is TriggerCharacter per LSP spec
		if ctx.TriggerKind == 2 {
			if ctx.TriggerCharacter != "" {
				for _, c := range s.triggerChars {
					if c == ctx.TriggerCharacter {
						return true
					}
				}
				return false
			}
			// No character provided but reported as TriggerCharacter; be conservative
			return false
		}
		// For TriggerForIncomplete (3), require manual char check below
	}
	// 2) Fallback: check the character immediately prior to cursor
	idx := p.Position.Character
	if idx <= 0 || idx > len(current) {
		return false
	}
	// Bare ';;' should not trigger via fallback char either
	if strings.Contains(current, ";;") && !hasDoubleSemicolonTrigger(current) {
		return false
	}
	ch := string(current[idx-1])
	for _, c := range s.triggerChars {
		if c == ch {
			return true
		}
	}
	return false
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
// LLM stats helpers moved to handlers_utils.go

// collectPromptRemovalEdits returns edits to remove all inline prompt markers.
// Supported form (inclusive):
//   - ";...;" where there is no space immediately after the first ';'
//     and no space immediately before the last ';'. An optional single space
//     after the trailing ';' is also removed for cleanliness.
//
// Multiple markers per line are supported.
// Inline prompt removal helpers moved to handlers_utils.go

// inParamList moved to handlers_utils.go

// buildPrompts moved to handlers_utils.go

// computeTextEditAndFilter moved to handlers_utils.go

// computeWordStart moved to handlers_utils.go

// isIdentChar moved to handlers_utils.go

// lineHasInlinePrompt returns true if the line contains an inline strict
// semicolon marker ;text; (no spaces at boundaries) or a double-semicolon
// pattern recognized by hasDoubleSemicolonTrigger.
// lineHasInlinePrompt moved to handlers_utils.go

// leadingIndent returns the run of leading spaces/tabs from the provided line.
// leadingIndent moved to handlers_utils.go

// applyIndent prefixes each non-empty line of suggestion with the given indent
// unless it already starts with that indent.
// applyIndent moved to handlers_utils.go

// isBareDoubleSemicolon reports whether the line contains a standalone
// double-semicolon marker with no inline content (";;" possibly with only
// whitespace after it). It explicitly excludes the valid form ";;text;".
// isBareDoubleSemicolon moved to handlers_utils.go

// stripDuplicateAssignmentPrefix removes a duplicated assignment prefix (e.g.,
// "name :=") from the beginning of the model suggestion when that same prefix
// already appears immediately to the left of the cursor on the current line.
// Also handles simple '=' assignments.
// stripDuplicateAssignmentPrefix moved to handlers_utils.go

// stripDuplicateGeneralPrefix removes any already-typed prefix that the model repeated
// at the beginning of its suggestion. It compares the entire text to the left of the
// cursor (prefixBeforeCursor) against the suggestion, trimming whitespace appropriately,
// and strips the longest sensible overlap. This prevents cases like:
//
//	prefix:    "func New "
//	suggestion:"func New() *Type"
//
// resulting in duplicates like "func New func New() *Type".
// stripDuplicateGeneralPrefix moved to handlers_utils.go

// isIdentBoundary moved to handlers_utils.go

// stripCodeFences removes surrounding Markdown code fences from a model
// response when the entire output is wrapped, e.g. starting with "```go" or
// "```" and ending with "```". It returns the inner content unchanged.
// stripCodeFences moved to handlers_utils.go

// stripInlineCodeSpan returns only the contents of the first inline backtick
// code span if present, e.g., "some text `x := y()` more" -> "x := y()".
// If no matching pair of backticks exists, it returns the input unchanged.
// This is intended for code completion responses where the model may wrap a
// small snippet in single backticks among prose.
// stripInlineCodeSpan moved to handlers_utils.go

// labelForCompletion moved to handlers_utils.go

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
