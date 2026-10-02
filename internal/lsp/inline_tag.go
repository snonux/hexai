// Locating inline prompt tags (">!text>" and ">>!text>") in document lines.

package lsp

// inlineTag is an inline prompt found on a line: the marker text with its byte
// span, and whether it is the line-replacing double-open form.
type inlineTag struct {
	text       string
	start, end int
	wholeLine  bool
}

// findInlineTag returns the first inline prompt tag on line.
func (s *Server) findInlineTag(line string) (inlineTag, bool) {
	openStr, _, openChar, closeChar := s.inlineMarkers()
	_, l, r, ok := findStrictInlineTag(line, openStr, openChar, closeChar)
	whole := hasDoubleOpenTrigger(line, openStr, openChar, closeChar)
	if !ok {
		if !whole {
			return inlineTag{}, false
		}
		l, r = 0, len(line)
	}
	return inlineTag{text: line[l:r], start: l, end: r, wholeLine: whole}, true
}

// findInlineTagLine returns the index of the line nearest to lineIdx whose
// inline prompt tag equals tagText, together with that tag, or -1.
func (s *Server) findInlineTagLine(lines []string, lineIdx int, tagText string) (int, inlineTag) {
	for dist := 0; dist < len(lines); dist++ {
		for _, i := range []int{lineIdx - dist, lineIdx + dist} {
			if i < 0 || i >= len(lines) {
				continue
			}
			if tag, ok := s.findInlineTag(lines[i]); ok && tag.text == tagText {
				return i, tag
			}
		}
	}
	return -1, inlineTag{}
}

// inlinePromptHandled reports whether the inline prompt on line is already
// being answered by the didChange-driven inline prompt runner.
func (cs *completionService) inlinePromptHandled(uri, line string) bool {
	tag, ok := cs.srv.findInlineTag(line)
	return ok && cs.srv.chatSvc().promptPending(inlineKey(uri, tag.text))
}

// inlineKey identifies an in-flight inline prompt by document and tag text.
func inlineKey(uri, tagText string) string {
	return promptKey(uri, "inline\x00"+tagText)
}
