// Summary: In-memory document model for the LSP; tracks text, lines, and applies edits.
// Not yet reviewed by a human
package lsp

import (
	"strings"
	"time"
)

// --- Document store and helpers ---

type document struct {
	uri   string
	text  string
	lines []string
}

func (s *Server) setDocument(uri, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs[uri] = &document{uri: uri, text: text, lines: splitLines(text)}
}

func (s *Server) deleteDocument(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, uri)
}

func (s *Server) markActivity() {
	s.mu.Lock()
	s.lastInput = time.Now()
	s.mu.Unlock()
}

func (s *Server) getDocument(uri string) *document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.docs[uri]
}

func splitLines(sx string) []string {
	sx = strings.ReplaceAll(sx, "\r\n", "\n")
	return strings.Split(sx, "\n")
}

func (s *Server) lineContext(uri string, pos Position) (above, current, below, funcCtx string) {
	d := s.getDocument(uri)
	if d == nil || len(d.lines) == 0 {
		return "", "", "", ""
	}
	idx := pos.Line
	if idx < 0 {
		idx = 0
	}
	if idx >= len(d.lines) {
		idx = len(d.lines) - 1
	}
	current = d.lines[idx]
	if idx-1 >= 0 {
		above = d.lines[idx-1]
	}
	if idx+1 < len(d.lines) {
		below = d.lines[idx+1]
	}
	for i := idx; i >= 0; i-- {
		line := strings.TrimSpace(d.lines[i])
		if hasAny(line, []string{"func ", "def ", "class ", "fn ", "procedure ", "sub "}) {
			funcCtx = line
			break
		}
	}
	return
}

// isDefiningNewFunction returns true when the cursor appears to be within
// a function declaration/signature and before the opening '{' of the body.
// Heuristic: find nearest preceding line containing "func "; ensure no '{'
// appears before the cursor across those lines.
func (s *Server) isDefiningNewFunction(uri string, pos Position) bool {
	d := s.getDocument(uri)
	if d == nil || len(d.lines) == 0 {
		return false
	}
	idx := pos.Line
	if idx < 0 {
		idx = 0
	}
	if idx >= len(d.lines) {
		idx = len(d.lines) - 1
	}
	// Find signature start
	sigStart := -1
	for i := idx; i >= 0; i-- {
		if strings.Contains(d.lines[i], "func ") {
			sigStart = i
			break
		}
		// stop if we hit a closing brace which likely ends a previous block
		if strings.Contains(d.lines[i], "}") {
			break
		}
	}
	if sigStart == -1 {
		return false
	}
	// Scan for '{' from sigStart up to cursor position; if found before or at cursor, we're in body
	for i := sigStart; i <= idx; i++ {
		line := d.lines[i]
		brace := strings.Index(line, "{")
		if brace >= 0 {
			if i < idx {
				return false // body started on a previous line
			}
			// same line as cursor: if brace position < cursor character, then already in body
			if pos.Character > brace {
				return false
			}
		}
	}
	return true
}

func hasAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func trimLen(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

func firstLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}
