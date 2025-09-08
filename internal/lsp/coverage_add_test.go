package lsp

import (
	"encoding/json"
	"testing"
)

func TestInParamListAndComputeWordStart(t *testing.T) {
	line := "func add(a int, b int) int { return a + b }"
	if !inParamList(line, 15) { // inside params
		t.Fatalf("expected inParamList true")
	}
	if inParamList("not a func", 3) {
		t.Fatalf("expected inParamList false")
	}
	if n := computeWordStart("helloWorld", 10); n != 0 {
		t.Fatalf("computeWordStart wrong: %d", n)
	}
}

func TestStripInlineAndLabel(t *testing.T) {
	if got := stripInlineCodeSpan("`abc`def"); got != "abc" {
		t.Fatalf("stripInlineCodeSpan: %q", got)
	}
	if lbl := labelForCompletion("First line\nSecond", "fir"); lbl != "First line" {
		t.Fatalf("labelForCompletion: %q", lbl)
	}
	if lbl := labelForCompletion("Other", "zzz"); lbl != "zzz" {
		t.Fatalf("label fallback: %q", lbl)
	}
}

func TestRangeComparators(t *testing.T) {
	a := Range{Start: Position{Line: 1, Character: 5}, End: Position{Line: 3, Character: 0}}
	b := Range{Start: Position{Line: 2, Character: 0}, End: Position{Line: 4, Character: 0}}
	if !rangesOverlap(a, b) {
		t.Fatalf("expected overlap")
	}
	if !lessPos(Position{Line: 1, Character: 0}, Position{Line: 1, Character: 1}) {
		t.Fatalf("lessPos")
	}
	if !greaterPos(Position{Line: 2, Character: 0}, Position{Line: 1, Character: 10}) {
		t.Fatalf("greaterPos")
	}
	if !isIdentChar('A') || isIdentChar('-') {
		t.Fatalf("isIdentChar")
	}
}

func TestFindGoFunctionAtLine_NoBody(t *testing.T) {
	lines := []string{"func X(a int)", "// comment"}
	start, end := findGoFunctionAtLine(lines, 0)
	if start != 0 || end != 0 {
		t.Fatalf("expected single-line prototype, got %d,%d", start, end)
	}
}

func TestLineHasInlinePrompt(t *testing.T) {
	if !lineHasInlinePrompt(">do>") {
		t.Fatalf("expected inline prompt")
	}
}

func TestDiagnosticsInRange_Overlap(t *testing.T) {
	s := &Server{}
	ctx := CodeActionContext{Diagnostics: []Diagnostic{{
		Range:   Range{Start: Position{Line: 10, Character: 0}, End: Position{Line: 12, Character: 0}},
		Message: "x",
	}}}
	raw, _ := json.Marshal(ctx)
	sel := Range{Start: Position{Line: 11, Character: 0}, End: Position{Line: 11, Character: 1}}
	out := s.diagnosticsInRange(raw, sel)
	if len(out) != 1 {
		t.Fatalf("expected 1 diag overlap, got %d", len(out))
	}
	// no diagnostics
	var empty json.RawMessage
	if o2 := s.diagnosticsInRange(empty, sel); len(o2) != 0 {
		t.Fatalf("expected 0 with empty ctx")
	}
}

func TestIndentHelpersAndPromptRemoval(t *testing.T) {
	if ind := leadingIndent("\t  ab"); ind == "" {
		t.Fatalf("expected indent")
	}
	if out := applyIndent("  ", "x\ny"); out != "  x\n  y" {
		t.Fatalf("applyIndent: %q", out)
	}
	// double-open trigger removes whole line
	edits := promptRemovalEditsForLine(">>ask>", 3)
	if len(edits) != 1 || edits[0].Range.Start.Line != 3 {
		t.Fatalf("unexpected edits: %#v", edits)
	}
	// temporarily switch to semicolon tags and test collection
	oldOpen, oldClose := inlineOpenChar, inlineCloseChar
	inlineOpenChar, inlineCloseChar = ';', ';'
	t.Cleanup(func() { inlineOpenChar, inlineCloseChar = oldOpen, oldClose })
	edits2 := collectSemicolonMarkers("pre;do;post", 1)
	if len(edits2) != 1 {
		t.Fatalf("expected one semicolon edit, got %#v", edits2)
	}
}
