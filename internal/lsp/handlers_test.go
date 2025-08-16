package lsp

import (
    "encoding/json"
    "strings"
    "testing"
)

func TestInParamList(t *testing.T) {
    line := "func foo(a int, b string) int {"
    if !inParamList(line, 15) { // inside params
        t.Fatalf("expected inParamList true for cursor inside params")
    }
    if inParamList(line, 2) { // before 'func'
        t.Fatalf("expected inParamList false for cursor before params")
    }
    if inParamList(line, len(line)) { // after ')'
        t.Fatalf("expected inParamList false for cursor after params")
    }
}

func TestComputeWordStart(t *testing.T) {
    current := "fmt.Prin"
    // Cursor after the word (index 8)
    got := computeWordStart(current, 8)
    // should stop after the dot at index 4
    if want := 4; got != want {
        t.Fatalf("computeWordStart got %d want %d", got, want)
    }
}

func TestComputeTextEditAndFilter_InParams(t *testing.T) {
    current := "func foo(a int, b string) {" // ')' at index 26
    p := CompletionParams{Position: Position{Line: 10, Character: 20}}
    te, filter := computeTextEditAndFilter("x int, y string", true, current, p)

    if te == nil {
        t.Fatalf("expected TextEdit")
    }
    // left should be after '(' which is at index 8
    if te.Range.Start.Line != 10 || te.Range.Start.Character != 9 {
        t.Fatalf("start got line=%d char=%d want line=10 char=9", te.Range.Start.Line, te.Range.Start.Character)
    }
    // right should clamp to cursor (20)
    if te.Range.End.Line != 10 || te.Range.End.Character != 20 {
        t.Fatalf("end got line=%d char=%d want line=10 char=20", te.Range.End.Line, te.Range.End.Character)
    }
    if filter == "" {
        t.Fatalf("expected non-empty filter inside params")
    }
}

func TestComputeTextEditAndFilter_Word(t *testing.T) {
    current := "fmt.Prin"
    p := CompletionParams{Position: Position{Line: 2, Character: len(current)}}
    te, filter := computeTextEditAndFilter("Println", false, current, p)
    if te == nil {
        t.Fatalf("expected TextEdit")
    }
    if te.Range.Start.Character != 4 || te.Range.End.Character != len(current) {
        t.Fatalf("range chars got %d..%d want 4..%d", te.Range.Start.Character, te.Range.End.Character, len(current))
    }
    if filter != "Prin" {
        t.Fatalf("filter got %q want %q", filter, "Prin")
    }
}

func TestLabelForCompletion(t *testing.T) {
    if got := labelForCompletion("Println", "Pri"); got != "Println" {
        t.Fatalf("label mismatch got %q want %q", got, "Println")
    }
    if got := labelForCompletion("Println", "X"); got != "X" {
        t.Fatalf("label mismatch with filter got %q want %q", got, "X")
    }
    if got := labelForCompletion("Println\nmore", ""); got != "Println" {
        t.Fatalf("label firstLine got %q want %q", got, "Println")
    }
}

func TestBuildPrompts_InParams(t *testing.T) {
    p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///t.go"}, Position: Position{Line: 1, Character: 12}}
    sys, user := buildPrompts(true, p, "above", "func foo(", "below", "func foo(")
    if sys == "" || user == "" {
        t.Fatalf("expected non-empty prompts")
    }
    if want := "function signatures"; !contains(sys, want) {
        t.Fatalf("system prompt missing %q: %q", want, sys)
    }
    if want := "parameter list"; !contains(user, want) {
        t.Fatalf("user prompt missing %q: %q", want, user)
    }
}

func TestBuildPrompts_Outside(t *testing.T) {
    p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///t.go"}, Position: Position{Line: 1, Character: 5}}
    sys, user := buildPrompts(false, p, "ab", "cur", "be", "fnctx")
    if sys == "" || user == "" {
        t.Fatalf("expected non-empty prompts")
    }
    if want := "completion engine"; !contains(sys, want) {
        t.Fatalf("system prompt missing %q: %q", want, sys)
    }
    if want := "Provide the next likely code"; !contains(user, want) {
        t.Fatalf("user prompt missing %q: %q", want, user)
    }
}

func TestComputeTextEditAndFilter_NoParensFallback(t *testing.T) {
    current := "func foo bar" // no parentheses
    cursor := len(current)
    p := CompletionParams{Position: Position{Line: 0, Character: cursor}}
    te, filter := computeTextEditAndFilter("baz", true, current, p)
    if te == nil {
        t.Fatalf("expected TextEdit from fallback path")
    }
    // fallback should behave like word edit; start at last space + 1
    lastSpace := strings.LastIndex(current, " ")
    if te.Range.Start.Character != lastSpace+1 || te.Range.End.Character != cursor {
        t.Fatalf("range got %d..%d want %d..%d", te.Range.Start.Character, te.Range.End.Character, lastSpace+1, cursor)
    }
    if filter != "bar" {
        t.Fatalf("filter got %q want %q", filter, "bar")
    }
}

// small helper to avoid importing strings
func contains(s, sub string) bool { return len(s) >= len(sub) && (func() bool { i := 0; for i+len(sub) <= len(s) { if s[i:i+len(sub)] == sub { return true }; i++ }; return false })() }




func TestCollectPromptRemovalEdits(t *testing.T) {
    s := newTestServer()
    uri := "file:///x.go"
    src := `keep ;tag; this and ;another; that
no markers here`
    s.setDocument(uri, src)
    edits := s.collectPromptRemovalEdits(uri)
    if len(edits) != 2 {
        t.Fatalf("expected 2 edits, got %d", len(edits))
    }
    // First occurrence ;tag;
    e0 := edits[0]
    if e0.Range.Start.Line != 0 {
        t.Fatalf("e0 start line=%d want 0", e0.Range.Start.Line)
    }
    if s.getDocument(uri).lines[0][e0.Range.Start.Character:e0.Range.Start.Character+1] != ";" {
        t.Fatalf("e0 start not at ;")
    }
}

func TestCollectPromptRemovalEdits_SkipSpacedMarkers(t *testing.T) {
    s := newTestServer()
    uri := "file:///y.go"
    // Only ;ok; should be removed; "; spaced ;" must be ignored
    src := `prefix ;ok; middle ; spaced ; suffix`
    s.setDocument(uri, src)
    edits := s.collectPromptRemovalEdits(uri)
    if len(edits) != 1 {
        t.Fatalf("expected 1 edit (only ;ok;), got %d", len(edits))
    }
    // Ensure the removed region starts at the first ';' of ;ok;
    line := s.getDocument(uri).lines[0]
    wantStart := strings.Index(line, ";ok;")
    if wantStart < 0 {
        t.Fatalf("test setup: could not find ;ok; in %q", line)
    }
    if edits[0].Range.Start.Line != 0 || edits[0].Range.Start.Character != wantStart {
        t.Fatalf("unexpected first edit start: got line=%d char=%d want line=0 char=%d", edits[0].Range.Start.Line, edits[0].Range.Start.Character, wantStart)
    }
}

func TestCollectPromptRemovalEdits_DoubleSemicolonRemovesWholeLine(t *testing.T) {
    s := newTestServer()
    uri := "file:///z.go"
    line0 := "keep"
    line1 := ";;todo; remove this whole line"
    line2 := "keep ;ok; end"
    src := strings.Join([]string{line0, line1, line2}, "\n")
    s.setDocument(uri, src)
    edits := s.collectPromptRemovalEdits(uri)
    if len(edits) != 2 {
        t.Fatalf("expected 2 edits (whole line + ;ok;), got %d", len(edits))
    }
    // Find the whole-line removal for line1
    found := false
    for _, e := range edits {
        if e.Range.Start.Line == 1 && e.Range.Start.Character == 0 && e.Range.End.Line == 1 && e.Range.End.Character == len(line1) {
            found = true
            break
        }
    }
    if !found {
        t.Fatalf("did not find whole-line removal edit for line 1")
    }
}

func TestCollectPromptRemovalEdits_SkipSpacedDouble(t *testing.T) {
    s := newTestServer()
    uri := "file:///w.go"
    src := "prefix ;; spaced ; suffix"
    s.setDocument(uri, src)
    edits := s.collectPromptRemovalEdits(uri)
    if len(edits) != 0 {
        t.Fatalf("expected 0 edits for spaced double-semicolon trigger, got %d", len(edits))
    }
}

func TestInstructionFromSelection_OrderPreference(t *testing.T) {
    // Earliest wins within a line
    line := "code /*block first*/ // later ;tag;"
    instr, cleaned := instructionFromSelection(line)
    if instr != "block first" {
        t.Fatalf("want block comment instr, got %q", instr)
    }
    if strings.Contains(cleaned, "block first") {
        t.Fatalf("cleaned should not contain the block comment")
    }
}

func TestInstructionFromSelection_SemicolonBeatsCommentIfEarlier(t *testing.T) {
    line := ";do this;// later"
    instr, cleaned := instructionFromSelection(line)
    if instr != "do this" {
        t.Fatalf("want semicolon instr, got %q", instr)
    }
    if strings.Contains(cleaned, ";do this;") {
        t.Fatalf("cleaned should have semicolon tag removed")
    }
}

func TestInstructionFromSelection_HTMLAndLineComments(t *testing.T) {
    line := "prefix <!-- html note --> suffix"
    instr, cleaned := instructionFromSelection(line)
    if instr != "html note" {
        t.Fatalf("want html note, got %q", instr)
    }
    if strings.Contains(cleaned, "<!--") || strings.Contains(cleaned, "-->") {
        t.Fatalf("cleaned should remove html comment markers")
    }
}

func TestStripDuplicateAssignmentPrefix(t *testing.T) {
    prefix := "matrix := "
    sug := "matrix := NewMatrix(2,2)"
    got := stripDuplicateAssignmentPrefix(prefix, sug)
    if got != "NewMatrix(2,2)" {
        t.Fatalf("dup strip failed: got %q", got)
    }
    // '=' variant
    prefix2 := "x = "
    sug2 := "x = y + 1"
    got2 := stripDuplicateAssignmentPrefix(prefix2, sug2)
    if got2 != "y + 1" {
        t.Fatalf("dup strip '=' failed: got %q", got2)
    }
}

func TestRangesOverlap(t *testing.T) {
    a := Range{Start: Position{Line: 1, Character: 2}, End: Position{Line: 3, Character: 0}}
    b := Range{Start: Position{Line: 2, Character: 0}, End: Position{Line: 4, Character: 1}}
    if !rangesOverlap(a, b) { t.Fatalf("expected overlap") }
    c := Range{Start: Position{Line: 4, Character: 1}, End: Position{Line: 5, Character: 0}}
    if rangesOverlap(a, c) { t.Fatalf("expected no overlap") }
}

func TestDiagnosticsInRange_Filtering(t *testing.T) {
    s := newTestServer()
    sel := Range{Start: Position{Line: 10, Character: 0}, End: Position{Line: 12, Character: 5}}
    // Build a fake context payload with three diagnostics: one inside, one outside, one touching boundary
    ctx := CodeActionContext{Diagnostics: []Diagnostic{
        {Range: Range{Start: Position{Line: 11, Character: 0}, End: Position{Line: 11, Character: 10}}, Message: "inside"},
        {Range: Range{Start: Position{Line: 2, Character: 0}, End: Position{Line: 3, Character: 0}}, Message: "outside"},
        {Range: Range{Start: Position{Line: 12, Character: 5}, End: Position{Line: 12, Character: 8}}, Message: "touch"},
    }}
    data, _ := json.Marshal(ctx)
    got := s.diagnosticsInRange(json.RawMessage(data), sel)
    if len(got) != 2 {
        t.Fatalf("expected 2 diagnostics in range, got %d", len(got))
    }
    msgs := []string{got[0].Message, got[1].Message}
    joined := strings.Join(msgs, ",")
    if !strings.Contains(joined, "inside") || !strings.Contains(joined, "touch") {
        t.Fatalf("unexpected diagnostics: %v", msgs)
    }
}
