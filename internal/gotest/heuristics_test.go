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

func TestFindFunctionAtLine_NoBody(t *testing.T) {
	lines := []string{"func X(a int)", "// comment"}
	start, end := FindFunctionAtLine(lines, 0)
	if start != 0 || end != 0 {
		t.Fatalf("expected single-line prototype, got %d,%d", start, end)
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

func TestExportName(t *testing.T) {
	if got := ExportName("sum"); got != "Sum" {
		t.Fatalf("got %q", got)
	}
	if got := ExportName("Sum"); got != "Sum" {
		t.Fatalf("got %q", got)
	}
}
