package textutil

import (
	"strings"
)

// SplitLinesBytes splits data into lines, handling both \n and \r\n endings.
// The returned slices share the underlying data array.
func SplitLinesBytes(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			end := i
			if end > start && data[end-1] == '\r' {
				end--
			}
			lines = append(lines, data[start:end])
			start = i + 1
		}
	}
	// Handle last line if it doesn't end with newline
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

// RenderTemplate performs simple {{var}} replacement in a template string.
func RenderTemplate(t string, vars map[string]string) string {
	if t == "" || len(vars) == 0 {
		return t
	}
	out := t
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

// StripCodeFences removes surrounding Markdown triple-backtick fences.
func StripCodeFences(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return t
	}
	lines := strings.Split(t, "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines) - 1
	for end >= 0 && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	if start >= len(lines) || end < 0 || start > end {
		return t
	}
	first := strings.TrimSpace(lines[start])
	last := strings.TrimSpace(lines[end])
	if strings.HasPrefix(first, "```") && last == "```" && end > start {
		inner := strings.Join(lines[start+1:end], "\n")
		return inner
	}
	return t
}

// InstructionFromSelection extracts the first inline instruction and returns
// (instruction, cleanedSelection). It detects markers on the earliest position
// per line in precedence: strict ;text;, /* */, <!-- -->, //, #, --.
func InstructionFromSelection(sel string) (string, string) {
	lines := strings.Split(sel, "\n")
	for idx, line := range lines {
		if instr, cleaned, ok := FindFirstInstructionInLine(line); ok && strings.TrimSpace(instr) != "" {
			lines[idx] = cleaned
			return instr, strings.Join(lines, "\n")
		}
	}
	return "", sel
}

// FindFirstInstructionInLine returns (instruction, cleaned, ok) for a single line.
func FindFirstInstructionInLine(line string) (instr, cleaned string, ok bool) {
	type cand struct {
		start, end int
		text       string
	}
	cands := []cand{}
	if t, l, r, ok := FindStrictInlineTag(line); ok {
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
	best := cands[0]
	for _, c := range cands[1:] {
		if c.start >= 0 && (best.start < 0 || c.start < best.start) {
			best = c
		}
	}
	cleaned = strings.TrimRight(line[:best.start]+line[best.end:], " \t")
	return best.text, cleaned, true
}

// FindStrictInlineTag finds ;text; with no spaces after/before semicolons.
func FindStrictInlineTag(line string) (text string, left, right int, ok bool) {
	for i := 0; i < len(line); i++ {
		if line[i] != ';' {
			continue
		}
		if i+1 < len(line) && line[i+1] == ' ' {
			continue
		}
		for j := i + 1; j < len(line); j++ {
			if line[j] == ';' {
				if j-1 >= 0 && line[j-1] == ' ' {
					continue
				}
				inner := strings.TrimSpace(line[i+1 : j])
				if inner != "" {
					return inner, i, j + 1, true
				}
			}
		}
	}
	return "", -1, -1, false
}
