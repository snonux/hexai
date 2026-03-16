// Builds additional context snippets based on configured mode and truncates text by token heuristic.
package lsp

import (
	"strings"

	"codeberg.org/snonux/hexai/internal/logging"
)

type contextModeBuilder func(*Server, bool, string, Position) (string, bool)

var contextModeRegistry = map[string]contextModeBuilder{
	"minimal":          minimalModeContext,
	"window":           windowModeContext,
	"file-on-new-func": fileOnNewFuncModeContext,
	"always-full":      alwaysFullModeContext,
}

// buildAdditionalContext builds extra context messages based on the configured mode.
// Modes:
// - minimal: no extra context
// - window: include a window of lines around the cursor
// - file-on-new-func: include full file only when defining a new function
// - always-full: always include the full file
func (s *Server) buildAdditionalContext(newFunc bool, uri string, pos Position) (string, bool) {
	builder, ok := contextModeRegistry[s.contextMode()]
	if !ok {
		// fallback to minimal if unknown
		return "", false
	}
	return builder(s, newFunc, uri, pos)
}

func minimalModeContext(_ *Server, _ bool, _ string, _ Position) (string, bool) {
	return "", false
}

func windowModeContext(s *Server, _ bool, uri string, pos Position) (string, bool) {
	return s.windowContext(uri, pos), true
}

func fileOnNewFuncModeContext(s *Server, newFunc bool, uri string, _ Position) (string, bool) {
	if !newFunc {
		return "", false
	}
	return s.fullFileContext(uri), true
}

func alwaysFullModeContext(s *Server, _ bool, uri string, _ Position) (string, bool) {
	return s.fullFileContext(uri), true
}

func (s *Server) windowContext(uri string, pos Position) string {
	d := s.getDocument(uri)
	if d == nil || len(d.lines) == 0 {
		logging.Logf("lsp ", "context: window requested but document not open; skipping uri=%s", uri)
		return ""
	}
	n := len(d.lines)
	half := s.windowLines() / 2
	start := pos.Line - half
	if start < 0 {
		start = 0
	}
	end := pos.Line + half + 1
	if end > n {
		end = n
	}
	text := strings.Join(d.lines[start:end], "\n")
	return truncateToApproxTokens(text, s.maxContextTokens())
}

func (s *Server) fullFileContext(uri string) string {
	d := s.getDocument(uri)
	if d == nil {
		logging.Logf("lsp ", "context: full-file requested but document not open; skipping uri=%s", uri)
		return ""
	}
	return truncateToApproxTokens(d.text, s.maxContextTokens())
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
