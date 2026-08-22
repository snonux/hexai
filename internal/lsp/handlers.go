// LSP JSON-RPC handlers; implements core methods and integrates with the LLM client when enabled.

package lsp

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/snonux/hexai/internal/logging"
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

// instructionFromSelection extracts the first instruction from selection text.
// Preference order on each line: strict ;text; marker (no inner spaces), then
// a line comment (//, #, --). Returns the instruction string and the selection
// text cleaned of the matched instruction marker or comment.
func (s *Server) instructionFromSelection(sel string) (string, string) {
	lines := splitLines(sel)
	for idx, line := range lines {
		if instr, cleaned, ok := s.findFirstInstructionInLine(line); ok && strings.TrimSpace(instr) != "" {
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
func (s *Server) findFirstInstructionInLine(line string) (instr string, cleaned string, ok bool) {
	type cand struct {
		start, end int
		text       string
	}
	cands := []cand{}
	openStr, _, openChar, closeChar := s.inlineMarkers()
	if t, l, r, ok := findStrictInlineTag(line, openStr, openChar, closeChar); ok {
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

func (s *Server) reply(id json.RawMessage, result any, err *RespError) {
	resp := Response{JSONRPC: "2.0", ID: id, Result: result, Error: err}
	s.writeMessage(resp)
}

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
	if client := s.currentLLMClient(); client != nil {
		prov = client.Name()
		model = client.DefaultModel()
	}
	temp := ""
	if tempPtr := s.codingTemperature(); tempPtr != nil {
		temp = fmt.Sprintf("%.3f", *tempPtr)
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

// checkTriggerFromContext inspects the LSP CompletionContext (if present) to decide if
// the completion was triggered by one of our configured trigger characters or by a manual
// invoke. Returns (result, decided): decided=true means the caller should use result
// directly; decided=false means the context was absent or inconclusive (TriggerKind 3).
func (s *Server) checkTriggerFromContext(p CompletionParams, current string, open string, openChar, closeChar byte, doubleSeqs, triggerChars []string) (result bool, decided bool) {
	if p.Context == nil {
		return false, false
	}
	var ctx struct {
		TriggerKind      int    `json:"triggerKind"`
		TriggerCharacter string `json:"triggerCharacter,omitempty"`
	}
	if raw, ok := p.Context.(json.RawMessage); ok {
		if err := json.Unmarshal(raw, &ctx); err != nil {
			logging.Logf("lsp ", "handleCompletion: unmarshal raw context: %v", err)
		}
	} else {
		b, err := json.Marshal(p.Context)
		if err != nil {
			logging.Logf("lsp ", "handleCompletion: marshal context: %v", err)
		} else if err := json.Unmarshal(b, &ctx); err != nil {
			logging.Logf("lsp ", "handleCompletion: unmarshal context: %v", err)
		}
	}
	// Bare double-open markers must not be treated as a trigger source.
	if containsAny(current, doubleSeqs) && !hasDoubleOpenTrigger(current, open, openChar, closeChar) {
		return false, true
	}
	// TriggerKind 1 = Invoked (manual). Always allow.
	if ctx.TriggerKind == 1 {
		return true, true
	}
	// TriggerKind 2 = TriggerCharacter per LSP spec.
	if ctx.TriggerKind == 2 {
		if ctx.TriggerCharacter != "" {
			for _, c := range triggerChars {
				if c == ctx.TriggerCharacter {
					return true, true
				}
			}
			return false, true
		}
		// No character provided but reported as TriggerCharacter; be conservative.
		return false, true
	}
	// TriggerKind 3 (TriggerForIncomplete): fall through to cursor-char check.
	return false, false
}

// checkTriggerFromCursorChar is the fallback check that looks at the character
// immediately to the left of the cursor position to decide whether it matches a
// configured trigger character.
func (s *Server) checkTriggerFromCursorChar(p CompletionParams, current string, open string, openChar, closeChar byte, doubleSeqs, triggerChars []string) bool {
	// Convert UTF-16 offset to byte offset for correct multi-byte handling.
	byteIdx := utf16OffsetToByteOffset(current, p.Position.Character)
	if byteIdx <= 0 || byteIdx > len(current) {
		return false
	}
	// Bare double-open should not trigger via fallback char check either.
	if containsAny(current, doubleSeqs) && !hasDoubleOpenTrigger(current, open, openChar, closeChar) {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(current[:byteIdx])
	ch := string(r)
	for _, c := range triggerChars {
		if c == ch {
			return true
		}
	}
	return false
}

// isTriggerEvent returns true when the completion request appears to be caused
// by typing one of our configured trigger characters. It checks the LSP
// CompletionContext if provided and also falls back to inspecting the character
// immediately to the left of the cursor.
func (s *Server) isTriggerEvent(p CompletionParams, current string) bool {
	open, _, openChar, closeChar := s.inlineMarkers()
	doubleSeqs := doubleOpenSequences(open, openChar, closeChar)
	triggerChars := s.triggerCharacters()
	// 1) Inspect LSP completion context if present.
	if result, decided := s.checkTriggerFromContext(p, current, open, openChar, closeChar, doubleSeqs, triggerChars); decided {
		return result
	}
	// 2) Fallback: check the character immediately prior to cursor.
	return s.checkTriggerFromCursorChar(p, current, open, openChar, closeChar, doubleSeqs, triggerChars)
}

func (s *Server) makeCompletionItems(cleaned string, inParams bool, current string, p CompletionParams, docStr string, detail string, sortPrefix string) []CompletionItem {
	te, filter := computeTextEditAndFilter(cleaned, inParams, current, p)
	rm := s.collectPromptRemovalEdits(p.TextDocument.URI)
	label := labelForCompletion(cleaned, filter)
	if strings.TrimSpace(detail) == "" {
		detail = "Hexai LLM completion"
	}
	if sortPrefix == "" {
		sortPrefix = "0000"
	}
	return []CompletionItem{{
		Label:               label,
		Kind:                1,
		Detail:              detail,
		InsertTextFormat:    1,
		FilterText:          strings.TrimLeft(filter, " \t"),
		TextEdit:            te,
		AdditionalTextEdits: rm,
		SortText:            sortPrefix,
		Documentation:       docStr,
	}}
}

func containsAny(haystack string, seqs []string) bool {
	for _, seq := range seqs {
		if seq == "" {
			continue
		}
		if strings.Contains(haystack, seq) {
			return true
		}
	}
	return false
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
