// Summary: Generic LSP helpers shared across handlers (LLM opts, prompts, text utils, counters).
package lsp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/logging"
	"codeberg.org/snonux/hexai/internal/stats"
	"codeberg.org/snonux/hexai/internal/textutil"
	tmx "codeberg.org/snonux/hexai/internal/tmux"
)

// llmRequestOpts builds request options from server settings.
func (s *Server) llmRequestOpts() []llm.RequestOption {
	maxTokens := s.maxTokens()
	client := s.currentLLMClient()
	tempPtr := s.codingTemperature()
	opts := []llm.RequestOption{llm.WithMaxTokens(maxTokens)}
	if tempPtr != nil {
		temp := *tempPtr
		if client != nil {
			prov := strings.ToLower(strings.TrimSpace(client.Name()))
			model := strings.ToLower(strings.TrimSpace(client.DefaultModel()))
			if prov == "openai" && strings.HasPrefix(model, "gpt-5") {
				temp = 1.0
			}
		}
		opts = append(opts, llm.WithTemperature(temp))
	}
	return opts
}

// small helpers for LLM traffic stats
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
	rpmLocal := float64(reqs) / mins
	sentPerMin := float64(sentTot) / mins
	recvPerMin := float64(recvTot) / mins
	// Log local process counters
	logging.Logf("lsp ", "llm stats (local) reqs=%d avg_sent=%d avg_recv=%d sent_total=%d recv_total=%d rpm=%.2f sent_per_min=%.0f recv_per_min=%.0f", reqs, avgSent, avgRecv, sentTot, recvTot, rpmLocal, sentPerMin, recvPerMin)
	// Global snapshot for tmux status
	snap, err := stats.TakeSnapshot()
	if err == nil {
		if client := s.currentLLMClient(); client != nil {
			provider := client.Name()
			model := client.DefaultModel()
			// Per-scope rpm estimated from window
			scopeReqs := int64(0)
			if pe, ok := snap.Providers[provider]; ok {
				if mc, ok2 := pe.Models[model]; ok2 {
					scopeReqs = mc.Reqs
				}
			}
			minsWin := snap.Window.Minutes()
			if minsWin <= 0 {
				minsWin = 0.001
			}
			scopeRPM := float64(scopeReqs) / minsWin
			status := tmx.FormatGlobalStatusColored(snap.Global.Reqs, snap.RPM, snap.Global.Sent, snap.Global.Recv, provider, model, scopeRPM, scopeReqs, snap.Window)
			_ = tmx.SetStatus(status)
		}
	}
}

// Completion prompt builders and filters
func inParamList(current string, cursor int) bool {
	if !strings.Contains(current, "func ") {
		return false
	}
	open := strings.Index(current, "(")
	close := strings.Index(current, ")")
	return open >= 0 && cursor > open && (close == -1 || cursor <= close)
}

// renderTemplate performs simple {{var}} replacement in a template string.
func renderTemplate(t string, vars map[string]string) string { return textutil.RenderTemplate(t, vars) }

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

// chatWithStats wraps llmClient.Chat to increment counters and emit a tmux heartbeat.
func (s *Server) chatWithStats(ctx context.Context, msgs []llm.Message, opts ...llm.RequestOption) (string, error) {
	// Count bytes sent
	sent := 0
	for _, m := range msgs {
		sent += len(m.Content)
	}
	s.incSentCounters(sent)
	// Debounce/throttle if configured (reuse completion gates)
	s.waitForDebounce(ctx)
	if !s.waitForThrottle(ctx) {
		return "", context.Canceled
	}
	// Perform request
	client := s.currentLLMClient()
	if client == nil {
		return "", fmt.Errorf("llm client unavailable")
	}
	txt, err := client.Chat(ctx, msgs, opts...)
	if err != nil {
		s.logLLMStats()
		return "", err
	}
	s.incRecvCounters(len(txt))
	// Update global stats cache
	_ = stats.Update(ctx, client.Name(), client.DefaultModel(), sent, len(txt))
	s.logLLMStats()
	return txt, nil
}

// Inline prompt utilities

func lineHasInlinePrompt(line string, open, close byte) bool {
	if _, _, _, ok := findStrictInlineTag(line, open, close); ok {
		return true
	}
	return hasDoubleOpenTrigger(line, open, close)
}

func leadingIndent(line string) string {
	i := 0
	for i < len(line) {
		if line[i] == ' ' || line[i] == '\t' {
			i++
			continue
		}
		break
	}
	if i == 0 {
		return ""
	}
	return line[:i]
}

func applyIndent(indent, suggestion string) string {
	if indent == "" || suggestion == "" {
		return suggestion
	}
	lines := splitLines(suggestion)
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if strings.HasPrefix(ln, indent) {
			continue
		}
		lines[i] = indent + ln
	}
	return strings.Join(lines, "\n")
}

// --- Inline marker parsing and general string utilities ---

// findStrictInlineTag finds >text> (configurable), with no space after the first
// opening marker and no space immediately before the closing marker. Returns the
// text between markers, the start index, the end index just after closing, and ok.
func findStrictInlineTag(line string, open, close byte) (string, int, int, bool) {
	pos := 0
	for pos < len(line) {
		// find opening marker
		j := strings.IndexByte(line[pos:], open)
		if j < 0 {
			return "", 0, 0, false
		}
		j += pos
		// ensure single open (not double) and non-space after
		if j+1 >= len(line) || line[j+1] == open || line[j+1] == ' ' {
			pos = j + 1
			continue
		}
		// find closing marker
		k := strings.IndexByte(line[j+1:], close)
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

// isBareDoubleSemicolon reports whether the line contains a standalone
// double-semicolon marker with no inline content (";;" possibly with only
// whitespace after it). It explicitly excludes the valid form ";;text;".
func isBareDoubleOpen(line string, open, close byte) bool {
	t := strings.TrimSpace(line)
	// check for double-open pattern
	dbl := string([]byte{open, open})
	if !strings.Contains(t, dbl) {
		return false
	}
	if hasDoubleOpenTrigger(t, open, close) {
		return false
	}
	if strings.HasPrefix(t, dbl) {
		rest := strings.TrimSpace(t[len(dbl):])
		if rest == "" || rest == ";" {
			return true
		}
	}
	return false
}

// stripDuplicateAssignmentPrefix removes a duplicated assignment prefix from the suggestion.
func stripDuplicateAssignmentPrefix(prefixBeforeCursor, suggestion string) string {
	s2 := strings.TrimLeft(suggestion, " \t")
	// Prefer := if present at end of prefix
	if idx := strings.LastIndex(prefixBeforeCursor, ":="); idx >= 0 && idx+2 <= len(prefixBeforeCursor) {
		tail := prefixBeforeCursor[idx+2:]
		if strings.TrimSpace(tail) == "" {
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
		if !(idx > 0 && prefixBeforeCursor[idx-1] == ':') { // not :=
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

// stripDuplicateGeneralPrefix removes any already-typed prefix that the model repeated.
func stripDuplicateGeneralPrefix(prefixBeforeCursor, suggestion string) string {
	if suggestion == "" {
		return suggestion
	}
	s := strings.TrimLeft(suggestion, " \t")
	p := strings.TrimRight(prefixBeforeCursor, " \t")
	if p != "" && strings.HasPrefix(s, p) {
		return strings.TrimLeft(s[len(p):], " \t")
	}
	for k := len(p) - 1; k > 0; k-- {
		if !isIdentBoundary(p[k-1]) {
			continue
		}
		suf := strings.TrimLeft(p[k:], " \t")
		if suf == "" {
			continue
		}
		if strings.HasPrefix(s, suf) {
			return strings.TrimLeft(s[len(suf):], " \t")
		}
	}
	return suggestion
}

func isIdentBoundary(ch byte) bool {
	return !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_')
}

// stripCodeFences removes surrounding Markdown code fences from a model response.
func stripCodeFences(s string) string { return textutil.StripCodeFences(s) }

// stripInlineCodeSpan returns the contents of the first inline backtick code span if present.
func stripInlineCodeSpan(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return t
	}
	i := strings.IndexByte(t, '`')
	if i < 0 {
		return t
	}
	jrel := strings.IndexByte(t[i+1:], '`')
	if jrel < 0 {
		return t
	}
	j := i + 1 + jrel
	return t[i+1 : j]
}

// labelForCompletion picks a short, readable label for the completion list.
func labelForCompletion(cleaned, filter string) string {
	label := trimLen(firstLine(cleaned))
	if filter != "" && !strings.HasPrefix(strings.ToLower(label), strings.ToLower(filter)) {
		return filter
	}
	return label
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

// collectPromptRemovalEdits returns edits to remove all inline prompt markers.
func (s *Server) collectPromptRemovalEdits(uri string) []TextEdit {
	d := s.getDocument(uri)
	if d == nil || len(d.lines) == 0 {
		return nil
	}
	var edits []TextEdit
	_, _, openChar, closeChar := s.inlineMarkers()
	for i, line := range d.lines {
		edits = append(edits, promptRemovalEditsForLine(line, i, openChar, closeChar)...)
	}
	return edits
}

func promptRemovalEditsForLine(line string, lineNum int, open, close byte) []TextEdit {
	if hasDoubleOpenTrigger(line, open, close) {
		return []TextEdit{{Range: Range{Start: Position{Line: lineNum, Character: 0}, End: Position{Line: lineNum, Character: len(line)}}, NewText: ""}}
	}
	return collectSemicolonMarkers(line, lineNum, open, close)
}

func hasDoubleOpenTrigger(line string, open, close byte) bool {
	pos := 0
	for pos < len(line) {
		// look for double-open sequence
		dbl := string([]byte{open, open})
		j := strings.Index(line[pos:], dbl)
		if j < 0 {
			return false
		}
		j += pos
		contentStart := j + len(dbl)
		if contentStart >= len(line) {
			return false
		}
		first := line[contentStart]
		if first == ' ' || first == open {
			pos = contentStart + 1
			continue
		}
		// find closing
		k := strings.IndexByte(line[contentStart+1:], close)
		if k < 0 {
			return false
		}
		closeIdx := contentStart + 1 + k
		if closeIdx-1 >= 0 && line[closeIdx-1] == ' ' {
			pos = closeIdx + 1
			continue
		}
		return true
	}
	return false
}

func collectSemicolonMarkers(line string, lineNum int, open, close byte) []TextEdit {
	var edits []TextEdit
	startSemi := 0
	for startSemi < len(line) {
		j := strings.IndexByte(line[startSemi:], open)
		if j < 0 {
			break
		}
		j += startSemi
		k := strings.IndexByte(line[j+1:], close)
		if k < 0 {
			break
		}
		if j+1 >= len(line) || line[j+1] == ' ' {
			startSemi = j + 1
			continue
		}
		if line[j+1] == open { // skip double-open start
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
