package lsp

import (
    "strings"
    "testing"
)

func TestHasDoubleSemicolonTrigger(t *testing.T) {
    cases := []struct{
        line string
        want bool
    }{
        {";;todo; remove this", true},
        {"prefix ;;x; suffix", true},
        {";; spaced ;", false},
        {"no markers", false},
        {";;x ; space before close", false},
    }
    for _, tc := range cases {
        got := hasDoubleSemicolonTrigger(tc.line)
        if got != tc.want {
            t.Fatalf("hasDoubleSemicolonTrigger(%q)=%v want %v", tc.line, got, tc.want)
        }
    }
}

func TestCollectSemicolonMarkers(t *testing.T) {
    line := "keep ;ok; this and ;another; that"
    edits := collectSemicolonMarkers(line, 7)
    if len(edits) != 2 {
        t.Fatalf("expected 2 edits, got %d", len(edits))
    }
    // Validate the first edit aligns with ;ok;
    start := strings.Index(line, ";ok;")
    if start < 0 { t.Fatalf("test setup: missing ;ok;") }
    if edits[0].Range.Start.Line != 7 || edits[0].Range.Start.Character != start {
        t.Fatalf("first edit start got line=%d char=%d want line=7 char=%d", edits[0].Range.Start.Line, edits[0].Range.Start.Character, start)
    }
}

func TestPromptRemovalEditsForLine_WholeLine(t *testing.T) {
    line := ";;todo; remove this whole line"
    edits := promptRemovalEditsForLine(line, 3)
    if len(edits) != 1 {
        t.Fatalf("expected 1 whole-line edit, got %d", len(edits))
    }
    e := edits[0]
    if e.Range.Start.Line != 3 || e.Range.End.Line != 3 || e.Range.Start.Character != 0 || e.Range.End.Character != len(line) {
        t.Fatalf("unexpected range for whole-line removal: %+v", e.Range)
    }
}

func TestStripCodeFences(t *testing.T) {
    cases := []struct{
        name string
        in   string
        want string
    }{
        {"no fences", "package main\nfunc x(){}", "package main\nfunc x(){}"},
        {"triple backticks no lang", "```\nA\nB\n```", "A\nB"},
        {"triple backticks with lang", "```go\nfmt.Println(\"hi\")\n```", "fmt.Println(\"hi\")"},
        {"leading/trailing spaces", " \n```python\nprint('x')\n```\n ", "print('x')"},
        {"single line fenced", "```go\npackage main\n```", "package main"},
    }
    for _, tc := range cases {
        got := stripCodeFences(tc.in)
        if got != tc.want {
            t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
        }
    }
}

func TestStripInlineCodeSpan(t *testing.T) {
    cases := []struct{
        name string
        in   string
        want string
    }{
        {"no backticks", "return x + y", "return x + y"},
        {"single inline", "Use `foo(bar)` here", "foo(bar)"},
        {"just inline", "`x := y()`", "x := y()"},
        {"unmatched start", "use `foo(bar) without end", "use `foo(bar) without end"},
        {"multiple spans picks first", "`a` and also `b`", "a"},
        {"leading/trailing spaces", "  text ` z `  ", " z "},
    }
    for _, tc := range cases {
        got := stripInlineCodeSpan(tc.in)
        if got != tc.want {
            t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
        }
    }
}
