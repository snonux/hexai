package lsp

import "testing"

func TestParseGoPackageName(t *testing.T) {
	lines := []string{"// comment", "package mypkg // trailing"}
	if got := parseGoPackageName(lines); got != "mypkg" {
		t.Fatalf("got %q", got)
	}
	if got := parseGoPackageName([]string{"no package"}); got != "" {
		t.Fatalf("expected empty")
	}
}

func TestDeriveGoFuncName(t *testing.T) {
	if got := deriveGoFuncName("func Sum(a int) int { return a }"); got != "Sum" {
		t.Fatalf("got %q", got)
	}
	if got := deriveGoFuncName("func (t *Type) Method(x int) {}"); got != "Method" {
		t.Fatalf("got %q", got)
	}
}
