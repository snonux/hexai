package gotest

import "testing"

func TestParsePackageName(t *testing.T) {
	lines := []string{"// comment", "package mypkg // trailing"}
	if got := ParsePackageName(lines); got != "mypkg" {
		t.Fatalf("got %q", got)
	}
	if got := ParsePackageName([]string{"no package"}); got != "" {
		t.Fatalf("expected empty package name")
	}
}

func TestParsePackageName_TabAfterName(t *testing.T) {
	// Covers the tab-trimming branch in ParsePackageName.
	lines := []string{"package mypkg\t// tab then comment"}
	if got := ParsePackageName(lines); got != "mypkg" {
		t.Fatalf("got %q, want %q", got, "mypkg")
	}
}

func TestParsePackageName_SpaceAfterName(t *testing.T) {
	// Covers the space-trimming branch (no comment, just trailing space).
	lines := []string{"package mypkg "}
	if got := ParsePackageName(lines); got != "mypkg" {
		t.Fatalf("got %q, want %q", got, "mypkg")
	}
}

func TestFindFunctionAtLine_NoBody(t *testing.T) {
	lines := []string{"func X(a int)", "// comment"}
	start, end := FindFunctionAtLine(lines, 0)
	if start != 0 || end != 0 {
		t.Fatalf("expected single-line prototype, got %d,%d", start, end)
	}
}

func TestFindFunctionAtLine_EmptyLines(t *testing.T) {
	// Covers the empty-lines early return.
	start, end := FindFunctionAtLine([]string{}, 0)
	if start != -1 || end != -1 {
		t.Fatalf("expected -1,-1 for empty input, got %d,%d", start, end)
	}
}

func TestFindFunctionAtLine_NegativeIdx(t *testing.T) {
	// Covers the idx < 0 clamping branch.
	lines := []string{"func Foo() {", "  return", "}"}
	start, end := FindFunctionAtLine(lines, -5)
	if start != 0 || end != 2 {
		t.Fatalf("expected 0,2 got %d,%d", start, end)
	}
}

func TestFindFunctionAtLine_IdxBeyondEnd(t *testing.T) {
	// Covers the idx >= len(lines) clamping branch.
	// The last line contains "func " so the backward scan finds it directly.
	lines := []string{"package main", "", "func Last() { }"}
	start, end := FindFunctionAtLine(lines, 100)
	if start != 2 || end != 2 {
		t.Fatalf("expected 2,2 got %d,%d", start, end)
	}
}

func TestFindFunctionAtLine_ClosingBraceBeforeFunc(t *testing.T) {
	// When scanning backward, hitting '}' before 'func ' means no enclosing function.
	lines := []string{"func A() {", "}", "  x := 1"}
	start, end := FindFunctionAtLine(lines, 2)
	if start != -1 || end != -1 {
		t.Fatalf("expected -1,-1 got %d,%d", start, end)
	}
}

func TestFindFunctionAtLine_NormalFunction(t *testing.T) {
	// Covers the normal path: finding a complete function with braces.
	lines := []string{
		"package main",
		"",
		"func Hello() {",
		"  fmt.Println(\"hi\")",
		"}",
	}
	start, end := FindFunctionAtLine(lines, 3)
	if start != 2 || end != 4 {
		t.Fatalf("expected 2,4 got %d,%d", start, end)
	}
}

func TestFindFunctionAtLine_UnclosedBrace(t *testing.T) {
	// Covers the branch where opening brace is seen but never closed.
	lines := []string{"func Broken() {", "  x := 1"}
	start, end := FindFunctionAtLine(lines, 0)
	if start != 0 || end != -1 {
		t.Fatalf("expected 0,-1 for unclosed brace, got %d,%d", start, end)
	}
}

func TestFindFunctionAtLine_NestedBraces(t *testing.T) {
	// Covers depth tracking with nested braces.
	lines := []string{
		"func Nested() {",
		"  if true {",
		"    x := 1",
		"  }",
		"}",
	}
	start, end := FindFunctionAtLine(lines, 2)
	if start != 0 || end != 4 {
		t.Fatalf("expected 0,4 got %d,%d", start, end)
	}
}

func TestDeriveFuncName(t *testing.T) {
	if got := DeriveFuncName("func Sum(a int) int { return a }"); got != "Sum" {
		t.Fatalf("got %q", got)
	}
	if got := DeriveFuncName("func (t *Type) Method(x int) {}"); got != "Method" {
		t.Fatalf("got %q", got)
	}
}

func TestDeriveFuncName_NotAFunc(t *testing.T) {
	// Covers the early return when line doesn't start with "func ".
	if got := DeriveFuncName("var x = 1"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestDeriveFuncName_MultiLine(t *testing.T) {
	// Covers the firstLine newline-splitting branch.
	code := "func Multi() {\n  return\n}"
	if got := DeriveFuncName(code); got != "Multi" {
		t.Fatalf("got %q, want %q", got, "Multi")
	}
}

func TestDeriveFuncName_MethodReceiverNoParenAfter(t *testing.T) {
	// Covers the case where receiver is parsed but no '(' follows the name.
	if got := DeriveFuncName("func (t *T) "); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExportName(t *testing.T) {
	if got := ExportName("sum"); got != "Sum" {
		t.Fatalf("got %q", got)
	}
	if got := ExportName("Sum"); got != "Sum" {
		t.Fatalf("got %q", got)
	}
}

func TestExportName_Empty(t *testing.T) {
	// Covers the empty-string early return.
	if got := ExportName(""); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
