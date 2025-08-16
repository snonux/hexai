package lsp

import (
	"hexai/internal/logging"
	"strings"
)

// buildAdditionalContext builds extra context messages based on the configured mode.
// Modes:
// - minimal: no extra context
// - window: include a window of lines around the cursor
// - file-on-new-func: include full file only when defining a new function
// - always-full: always include the full file
func (s *Server) buildAdditionalContext(newFunc bool, uri string, pos Position) (string, bool) {
	mode := s.contextMode
	switch mode {
	case "minimal":
		return "", false
	case "window":
		return s.windowContext(uri, pos), true
	case "file-on-new-func":
		if newFunc {
			return s.fullFileContext(uri), true
		}
		return "", false
	case "always-full":
		return s.fullFileContext(uri), true
	default:
		// fallback to minimal if unknown
		return "", false
	}
}

func (s *Server) windowContext(uri string, pos Position) string {
	d := s.getDocument(uri)
	if d == nil || len(d.lines) == 0 {
		logging.Logf("lsp ", "context: window requested but document not open; skipping uri=%s", uri)
		return ""
	}
	n := len(d.lines)
	half := s.windowLines / 2
	start := pos.Line - half
	if start < 0 {
		start = 0
	}
	end := pos.Line + half + 1
	if end > n {
		end = n
	}
	text := strings.Join(d.lines[start:end], "\n")
	return truncateToApproxTokens(text, s.maxContextTokens)
}

func (s *Server) fullFileContext(uri string) string {
	d := s.getDocument(uri)
	if d == nil {
		logging.Logf("lsp ", "context: full-file requested but document not open; skipping uri=%s", uri)
		return ""
	}
	return truncateToApproxTokens(d.text, s.maxContextTokens)
}

// truncateToApproxTokens naively truncates the input to fit approx N tokens.
// Uses 4 chars/token heuristic for speed and determinism.
func truncateToApproxTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	// try to cut on a line boundary near maxChars
	cut := maxChars
	if cut > len(text) {
		cut = len(text)
	}
	if i := strings.LastIndex(text[:cut], "\n"); i > 0 {
		cut = i
	}
	return text[:cut]
}
