// Summary: Generic LSP helpers shared across handlers (LLM opts, prompts, text utils, counters).
package lsp

import (
	"fmt"
	"hexai/internal/llm"
	"hexai/internal/logging"
	"strings"
	"time"
)

// llmRequestOpts builds request options from server settings.
func (s *Server) llmRequestOpts() []llm.RequestOption {
	opts := []llm.RequestOption{llm.WithMaxTokens(s.maxTokens)}
	if s.codingTemperature != nil {
		opts = append(opts, llm.WithTemperature(*s.codingTemperature))
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
	rpm := float64(reqs) / mins
	sentPerMin := float64(sentTot) / mins
	recvPerMin := float64(recvTot) / mins
	logging.Logf("lsp ", "llm stats reqs=%d avg_sent=%d avg_recv=%d sent_total=%d recv_total=%d rpm=%.2f sent_per_min=%.0f recv_per_min=%.0f", reqs, avgSent, avgRecv, sentTot, recvTot, rpm, sentPerMin, recvPerMin)
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

// Inline prompt utilities
func lineHasInlinePrompt(line string) bool {
	if _, _, _, ok := findStrictSemicolonTag(line); ok {
		return true
	}
	return hasDoubleSemicolonTrigger(line)
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

// collectPromptRemovalEdits returns edits to remove all inline prompt markers.
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
		contentStart := j + 2
		if contentStart >= len(line) {
			return false
		}
		first := line[contentStart]
		if first == ' ' || first == ';' {
			pos = contentStart + 1
			continue
		}
		k := strings.Index(line[contentStart+1:], ";")
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
