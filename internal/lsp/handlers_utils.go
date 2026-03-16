// Summary: Generic LSP helpers shared across handlers (LLM opts, prompts, text utils, counters).
package lsp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/llmutils"
	"codeberg.org/snonux/hexai/internal/logging"
	"codeberg.org/snonux/hexai/internal/stats"
	"codeberg.org/snonux/hexai/internal/textutil"
)

type surfaceKind string

const (
	surfaceCompletion surfaceKind = "completion"
	surfaceCodeAction surfaceKind = "code_action"
	surfaceChat       surfaceKind = "chat"
)

type requestSpec struct {
	provider      string
	entry         appconfig.SurfaceConfig
	fallbackModel string
	options       []llm.RequestOption
	index         int
}

func (r requestSpec) effectiveModel(defaultModel string) string {
	if m := strings.TrimSpace(r.entry.Model); m != "" {
		return m
	}
	if f := strings.TrimSpace(r.fallbackModel); f != "" {
		return f
	}
	return strings.TrimSpace(defaultModel)
}

func (s *Server) buildRequestSpecs(surface surfaceKind) []requestSpec {
	cfg := s.currentConfig()
	entries := surfaceConfigsFor(cfg, surface)
	if len(entries) == 0 {
		entries = []appconfig.SurfaceConfig{{Provider: cfg.Provider}}
	}
	maxTokens := s.maxTokens()
	specs := make([]requestSpec, 0, len(entries))
	for idx, raw := range entries {
		entry := appconfig.SurfaceConfig{
			Provider:    strings.TrimSpace(raw.Provider),
			Model:       strings.TrimSpace(raw.Model),
			Temperature: raw.Temperature,
		}
		provider := entry.Provider
		if provider == "" {
			provider = cfg.Provider
		}
		provider = canonicalProvider(provider)
		fallbackModel := entry.Model
		if fallbackModel == "" {
			fallbackModel = strings.TrimSpace(llmutils.DefaultModelForProvider(cfg, provider))
		}
		opts := []llm.RequestOption{llm.WithMaxTokens(maxTokens)}
		if entry.Model != "" {
			opts = append(opts, llm.WithModel(entry.Model))
		}
		if temp, ok := chooseSurfaceTemperature(cfg, entry, provider, fallbackModel); ok {
			opts = append(opts, llm.WithTemperature(temp))
		}
		specs = append(specs, requestSpec{
			provider:      provider,
			entry:         entry,
			fallbackModel: fallbackModel,
			options:       opts,
			index:         idx,
		})
	}
	return specs
}

func (s *Server) primaryRequestSpec(surface surfaceKind) requestSpec {
	specs := s.buildRequestSpecs(surface)
	if len(specs) == 0 {
		cfg := s.currentConfig()
		provider := canonicalProvider(cfg.Provider)
		fallback := strings.TrimSpace(llmutils.DefaultModelForProvider(cfg, provider))
		return requestSpec{provider: provider, fallbackModel: fallback, options: []llm.RequestOption{llm.WithMaxTokens(s.maxTokens())}}
	}
	return specs[0]
}

// buildRequestSpec is retained for consumers expecting a single-entry helper.
func (s *Server) buildRequestSpec(surface surfaceKind) requestSpec {
	return s.primaryRequestSpec(surface)
}

func canonicalProvider(name string) string {
	return llmutils.CanonicalProvider(name)
}

func surfaceConfigsFor(cfg appconfig.App, surface surfaceKind) []appconfig.SurfaceConfig {
	switch surface {
	case surfaceCompletion:
		return cfg.CompletionConfigs
	case surfaceCodeAction:
		return cfg.CodeActionConfigs
	case surfaceChat:
		return cfg.ChatConfigs
	default:
		return nil
	}
}

// chooseSurfaceTemperature resolves the effective temperature for a surface
// request, delegating GPT-5 override logic to llmutils.ResolveTemperature.
func chooseSurfaceTemperature(cfg appconfig.App, entry appconfig.SurfaceConfig, provider string, fallbackModel string) (float64, bool) {
	effectiveModel := strings.TrimSpace(entry.Model)
	if effectiveModel == "" {
		effectiveModel = strings.TrimSpace(fallbackModel)
	}
	return llmutils.ResolveTemperature(provider, effectiveModel, entry.Temperature, cfg.CodingTemperature)
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

func (s *Server) logLLMStats(model string) {
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
			modelName := strings.TrimSpace(model)
			if modelName == "" {
				modelName = client.DefaultModel()
			}
			// Per-scope rpm estimated from window
			scopeReqs := int64(0)
			if pe, ok := snap.Providers[provider]; ok {
				if mc, ok2 := pe.Models[modelName]; ok2 {
					scopeReqs = mc.Reqs
				}
			}
			minsWin := snap.Window.Minutes()
			if minsWin <= 0 {
				minsWin = 0.001
			}
			scopeRPM := float64(scopeReqs) / minsWin
			s.emitGlobalStatus(snap.Global.Reqs, snap.RPM, snap.Global.Sent, snap.Global.Recv, provider, modelName, scopeRPM, scopeReqs, snap.Window)
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
func (s *Server) chatWithStats(ctx context.Context, surface surfaceKind, spec requestSpec, msgs []llm.Message) (string, error) {
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
	client := s.clientFor(spec)
	if client == nil {
		return "", fmt.Errorf("llm client unavailable")
	}
	modelUsed := spec.effectiveModel(client.DefaultModel())
	txt, err := client.Chat(ctx, msgs, spec.options...)
	if err != nil {
		s.logLLMStats(modelUsed)
		return "", err
	}
	s.incRecvCounters(len(txt))
	// Update global stats cache
	_ = stats.Update(ctx, client.Name(), modelUsed, sent, len(txt))
	s.logLLMStats(modelUsed)
	return txt, nil
}

// Inline prompt utilities

func lineHasInlinePrompt(line string, openStr string, open, close byte) bool {
	if openStr == "" {
		openStr = string(open)
	}
	if _, _, _, ok := findStrictInlineTag(line, openStr, open, close); ok {
		return true
	}
	return hasDoubleOpenTrigger(line, openStr, open, close)
}

func doubleOpenSequences(openStr string, open, close byte) []string {
	seen := make(map[string]struct{}, 2)
	var seqs []string
	if openStr != "" && close != 0 {
		seq := openStr + string(close)
		if _, ok := seen[seq]; !ok {
			seen[seq] = struct{}{}
			seqs = append(seqs, seq)
		}
	}
	if openStr != "" && open != 0 {
		seq := string(open) + openStr
		if len(seq) > len(openStr) {
			if _, ok := seen[seq]; !ok {
				seen[seq] = struct{}{}
				seqs = append(seqs, seq)
			}
		}
	}
	return seqs
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

// findStrictInlineTag finds >!text> (configurable), with no space after the first
// opening marker and no space immediately before the closing marker. Returns the
// text between markers, the start index, the end index just after closing, and ok.
func findStrictInlineTag(line string, openStr string, open, close byte) (string, int, int, bool) {
	if openStr == "" {
		openStr = string(open)
	}
	if openStr == "" {
		return "", 0, 0, false
	}
	openChar := open
	if openChar == 0 {
		openChar = openStr[0]
	}
	doubleSeqs := doubleOpenSequences(openStr, openChar, close)
	pos := 0
	for pos < len(line) {
		j := strings.IndexByte(line[pos:], openChar)
		if j < 0 {
			return "", 0, 0, false
		}
		j += pos
		if !strings.HasPrefix(line[j:], openStr) {
			pos = j + 1
			continue
		}
		contentStart := j + len(openStr)
		if contentStart >= len(line) {
			return "", 0, 0, false
		}
		doubleHit := false
		for _, seq := range doubleSeqs {
			if strings.HasPrefix(line[j:], seq) {
				doubleHit = true
				contentStart += len(seq) - len(openStr)
				if contentStart >= len(line) {
					return "", 0, 0, false
				}
				break
			}
		}
		next := line[contentStart]
		if next == ' ' {
			pos = contentStart + 1
			continue
		}
		if !doubleHit && next == close {
			pos = contentStart + 1
			continue
		}
		k := strings.IndexByte(line[contentStart:], close)
		if k < 0 {
			return "", 0, 0, false
		}
		closeIdx := contentStart + k
		if closeIdx-1 >= contentStart && line[closeIdx-1] == ' ' {
			pos = closeIdx + 1
			continue
		}
		inner := strings.TrimSpace(line[contentStart:closeIdx])
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
func isBareDoubleOpen(line string, openStr string, open, close byte) bool {
	t := strings.TrimSpace(line)
	if openStr == "" {
		openStr = string(open)
	}
	if openStr == "" {
		return false
	}
	for _, seq := range doubleOpenSequences(openStr, open, close) {
		if strings.HasPrefix(t, seq) {
			rest := strings.TrimSpace(t[len(seq):])
			if rest == "" || rest == string(close) {
				return true
			}
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
		if idx <= 0 || prefixBeforeCursor[idx-1] != ':' { // not :=
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
	return !isIdentChar(ch)
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
// It performs bounds checks on line indices and character offsets, returning
// an empty string when the range is invalid (e.g. negative lines, out-of-bounds
// lines, or an empty document).
func extractRangeText(d *document, r Range) string {
	if d == nil || len(d.lines) == 0 {
		return ""
	}
	// Clamp line indices to valid bounds.
	if r.Start.Line < 0 || r.End.Line < 0 || r.Start.Line >= len(d.lines) {
		return ""
	}
	if r.End.Line >= len(d.lines) {
		r.End.Line = len(d.lines) - 1
		r.End.Character = len(d.lines[r.End.Line])
	}
	if r.Start.Line > r.End.Line {
		return ""
	}

	if r.Start.Line == r.End.Line {
		return extractSingleLineRange(d.lines[r.Start.Line], r)
	}
	return extractMultiLineRange(d.lines, r)
}

// extractSingleLineRange handles the case where start and end are on the same line.
// Character offsets are clamped to the line length.
func extractSingleLineRange(line string, r Range) string {
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

// extractMultiLineRange handles ranges spanning multiple lines, clamping
// character offsets on the first and last lines.
func extractMultiLineRange(lines []string, r Range) string {
	var b strings.Builder
	// first line
	first := lines[r.Start.Line]
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
		b.WriteString(lines[i])
		if i+1 <= r.End.Line {
			b.WriteString("\n")
		}
	}
	// last line
	last := lines[r.End.Line]
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
	openStr, _, openChar, closeChar := s.inlineMarkers()
	for i, line := range d.lines {
		edits = append(edits, promptRemovalEditsForLine(line, i, openStr, openChar, closeChar)...)
	}
	return edits
}

func promptRemovalEditsForLine(line string, lineNum int, openStr string, open, close byte) []TextEdit {
	if hasDoubleOpenTrigger(line, openStr, open, close) {
		return []TextEdit{{Range: Range{Start: Position{Line: lineNum, Character: 0}, End: Position{Line: lineNum, Character: len(line)}}, NewText: ""}}
	}
	return collectSemicolonMarkers(line, lineNum, openStr, open, close)
}

func hasDoubleOpenTrigger(line string, openStr string, open, close byte) bool {
	if openStr == "" {
		openStr = string(open)
	}
	if openStr == "" {
		return false
	}
	seqs := doubleOpenSequences(openStr, open, close)
	if len(seqs) == 0 {
		return false
	}
	pos := 0
	for pos < len(line) {
		found := -1
		var seq string
		for _, cand := range seqs {
			if cand == "" {
				continue
			}
			if idx := strings.Index(line[pos:], cand); idx >= 0 {
				abs := pos + idx
				if found < 0 || abs < found {
					found = abs
					seq = cand
				}
			}
		}
		if found < 0 {
			return false
		}
		contentStart := found + len(seq)
		if contentStart >= len(line) {
			return false
		}
		first := line[contentStart]
		if first == ' ' || first == close || first == open {
			pos = contentStart + 1
			continue
		}
		if contentStart+1 >= len(line) {
			return false
		}
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

func collectSemicolonMarkers(line string, lineNum int, openStr string, open, close byte) []TextEdit {
	if openStr == "" {
		openStr = string(open)
	}
	if openStr == "" {
		return nil
	}
	var edits []TextEdit
	start := 0
	doubleSeqs := doubleOpenSequences(openStr, open, close)
	for start < len(line) {
		j := strings.Index(line[start:], openStr)
		if j < 0 {
			break
		}
		j += start
		contentStart := j + len(openStr)
		if contentStart >= len(line) {
			break
		}
		next := line[contentStart]
		if next == ' ' {
			start = j + 1
			continue
		}
		skipDouble := false
		for _, seq := range doubleSeqs {
			if strings.HasPrefix(line[j:], seq) {
				skipDouble = true
				break
			}
		}
		if skipDouble {
			start = j + 1
			continue
		}
		k := strings.IndexByte(line[contentStart:], close)
		if k < 0 {
			break
		}
		closeIdx := contentStart + k
		if closeIdx-1 < contentStart || line[closeIdx-1] == ' ' {
			start = closeIdx + 1
			continue
		}
		if closeIdx == contentStart {
			start = closeIdx + 1
			continue
		}
		endChar := closeIdx + 1
		if endChar < len(line) && line[endChar] == ' ' {
			endChar++
		}
		edits = append(edits, TextEdit{Range: Range{Start: Position{Line: lineNum, Character: j}, End: Position{Line: lineNum, Character: endChar}}, NewText: ""})
		start = endChar
	}
	return edits
}
