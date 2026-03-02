package gotest

import "strings"

// ParsePackageName returns the package name from file lines, or empty when missing.
func ParsePackageName(lines []string) string {
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if !strings.HasPrefix(t, "package ") {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(t, "package "))
		if i := strings.Index(name, " "); i >= 0 {
			name = name[:i]
		}
		if i := strings.Index(name, "\t"); i >= 0 {
			name = name[:i]
		}
		if i := strings.Index(name, "//"); i >= 0 {
			name = strings.TrimSpace(name[:i])
		}
		return name
	}
	return ""
}

// FindFunctionAtLine finds the function enclosing or preceding idx.
// It returns start/end line indexes or -1/-1 when no function is found.
func FindFunctionAtLine(lines []string, idx int) (int, int) {
	if len(lines) == 0 {
		return -1, -1
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(lines) {
		idx = len(lines) - 1
	}

	start := -1
	for i := idx; i >= 0; i-- {
		if strings.Contains(lines[i], "func ") {
			start = i
			break
		}
		if strings.Contains(lines[i], "}") {
			break
		}
	}
	if start == -1 {
		return -1, -1
	}

	depth := 0
	seenOpen := false
	for i := start; i < len(lines); i++ {
		line := lines[i]
		for j := 0; j < len(line); j++ {
			switch line[j] {
			case '{':
				depth++
				seenOpen = true
			case '}':
				if depth > 0 {
					depth--
				}
				if seenOpen && depth == 0 {
					return start, i
				}
			}
		}
	}
	if !seenOpen {
		return start, start
	}
	return start, -1
}

// DeriveFuncName extracts function or method name from Go source snippet.
func DeriveFuncName(code string) string {
	line := strings.TrimSpace(firstLine(code))
	if !strings.HasPrefix(line, "func ") {
		return ""
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "func "))
	if strings.HasPrefix(rest, "(") {
		if i := strings.Index(rest, ")"); i >= 0 && i+1 < len(rest) {
			rest = strings.TrimSpace(rest[i+1:])
		}
	}
	if i := strings.Index(rest, "("); i > 0 {
		return strings.TrimSpace(rest[:i])
	}
	return ""
}

// ExportName upper-cases the first character for use in Test* names.
func ExportName(name string) string {
	if name == "" {
		return ""
	}
	r := []rune(name)
	first := string(r[0])
	r[0] = []rune(strings.ToUpper(first))[0]
	return string(r)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
