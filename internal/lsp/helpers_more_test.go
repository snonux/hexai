package lsp

import ("testing")

func TestComputeWordStart(t *testing.T) {
    s := "fooBar 123"
    if i := computeWordStart(s, 5); i != 0 { t.Fatalf("start=%d", i) }
    if i := computeWordStart(s, len(s)); i != 7 { t.Fatalf("end start=%d", i) }
}

func TestLeadingAndApplyIndent(t *testing.T) {
    if got := leadingIndent("\t  abc"); got == "" { t.Fatalf("expected indent") }
    out := applyIndent("  ", "x\n y\n\n z")
    if out == "" || out[:2] != "  " { t.Fatalf("applyIndent failed: %q", out) }
}

func TestFindStrictSemicolonTag(t *testing.T) {
    if _, _, _, ok := findStrictSemicolonTag(";do this; next"); !ok { t.Fatalf("expected strict tag") }
    if _, _, _, ok := findStrictSemicolonTag("; spaced ;"); ok { t.Fatalf("should ignore spaced tag") }
}

// hasDoubleSemicolonTrigger tested elsewhere

func TestStripDuplicatePrefixes(t *testing.T) {
    if got := stripDuplicateAssignmentPrefix("name := ", "name := 123"); got == "name := 123" { t.Fatalf("expected trim") }
    if got := stripDuplicateGeneralPrefix("fmt.", "fmt.Println"); got == "fmt.Println" { t.Fatalf("expected trim general") }
}

func TestExtractRangeText(t *testing.T) {
    d := &document{text: "a\nbc\nxyz", lines: []string{"a","bc","xyz"}}
    // single line
    got := extractRangeText(d, Range{Start: Position{Line:1, Character:0}, End: Position{Line:1, Character:2}})
    if got != "bc" { t.Fatalf("got %q", got) }
    // multi-line
    got = extractRangeText(d, Range{Start: Position{Line:0, Character:0}, End: Position{Line:2, Character:2}})
    if got != "a\nbc\nxy" { t.Fatalf("got %q", got) }
}

func TestRangesOverlapAndOrder(t *testing.T) {
    a := Range{Start: Position{Line:1, Character:2}, End: Position{Line:1, Character:5}}
    b := Range{Start: Position{Line:1, Character:4}, End: Position{Line:1, Character:8}}
    if !rangesOverlap(a, b) { t.Fatalf("expected overlap") }
    c := Range{Start: Position{Line:2, Character:0}, End: Position{Line:2, Character:1}}
    if rangesOverlap(a, c) { t.Fatalf("no overlap expected") }
    if !lessPos(Position{Line:0, Character:1}, Position{Line:1, Character:0}) { t.Fatalf("lessPos failed") }
    if !greaterPos(Position{Line:2, Character:0}, Position{Line:1, Character:9}) { t.Fatalf("greaterPos failed") }
}

func TestPromptRemovalEditsForLine(t *testing.T) {
    edits := promptRemovalEditsForLine(";;do thing;", 3)
    if len(edits) != 1 || edits[0].Range.Start.Line != 3 {
        t.Fatalf("expected full-line removal for double-semicolon")
    }
    edits2 := promptRemovalEditsForLine(";act; and ;b;", 1)
    if len(edits2) == 0 { t.Fatalf("expected edits to remove strict markers") }
}

func TestCollectPromptRemovalEdits_MultiLine(t *testing.T) {
    s := newTestServer()
    uri := "file:///t.go"
    s.setDocument(uri, "a\n;do; x\n;;wipe;\nend")
    edits := s.collectPromptRemovalEdits(uri)
    if len(edits) < 2 { t.Fatalf("expected >=2 edits, got %d", len(edits)) }
}

func TestInParamListAndBuildPrompts(t *testing.T) {
    cur := "func add(a int, b string) int"
    if !inParamList(cur, 12) { t.Fatalf("expected in param list") }
    p := CompletionParams{TextDocument: TextDocumentIdentifier{URI: "file:///x.go"}, Position: Position{Line: 0, Character: 5}}
    sys, user := buildPrompts(false, p, "above", "current", "below", "func add")
    if sys == "" || user == "" { t.Fatalf("prompts empty") }
}

func TestLabelForCompletion(t *testing.T) {
    if got := labelForCompletion("line one\nline two", "lin"); got != "line one" { t.Fatalf("expected label, got %q", got) }
    if got := labelForCompletion("result", "zzz"); got != "zzz" { t.Fatalf("expected filter preferred when not prefix, got %q", got) }
    if got := labelForCompletion("result", "re"); got != "result" { t.Fatalf("expected label when filter prefixes label, got %q", got) }
}

func TestComputeTextEditAndFilter(t *testing.T) {
    // non-params edit
    p := CompletionParams{Position: Position{Line: 1, Character: 4}}
    te, filter := computeTextEditAndFilter("X", false, "ab cd", p)
    if te == nil || filter == "" { t.Fatalf("expected edit and filter") }
    // inside params
    line := "func add(a int, b int)"
    p2 := CompletionParams{Position: Position{Line: 0, Character: 12}}
    te2, _ := computeTextEditAndFilter("string", true, line, p2)
    if te2 == nil || te2.Range.Start.Character == 0 { t.Fatalf("expected param-range edit") }
}

func TestIsBareDoubleSemicolon(t *testing.T) {
    if !isBareDoubleSemicolon(";;   ") { t.Fatalf("expected true") }
    if isBareDoubleSemicolon(";;x;") { t.Fatalf("expected false for content form") }
}

func TestIsDefiningNewFunction(t *testing.T) {
    s := newTestServer()
    uri := "file:///z.go"
    s.setDocument(uri, "package p\n\nfunc add(a int) int\n{")
    if !s.isDefiningNewFunction(uri, Position{Line:2, Character:10}) {
        t.Fatalf("expected true before opening brace")
    }
    if s.isDefiningNewFunction(uri, Position{Line:3, Character:1}) {
        t.Fatalf("expected false inside body")
    }
}
