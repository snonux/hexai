// Summary: Tests for instruction extraction helpers in handlers.go
// Not yet reviewed by a human
package lsp

import "testing"

func TestFindFirstInstructionInLine_NoMarker(t *testing.T) {
    line := "fmt.Println(\"hello\")"
    instr, cleaned, ok := findFirstInstructionInLine(line)
    if ok {
        t.Fatalf("expected ok=false; got ok=true with instr=%q cleaned=%q", instr, cleaned)
    }
    if instr != "" || cleaned != line {
        t.Fatalf("unexpected outputs: instr=%q cleaned=%q", instr, cleaned)
    }
}

func TestFindFirstInstructionInLine_StrictSemicolon_Basic(t *testing.T) {
    line := "prefix ;rename var; suffix"
    instr, cleaned, ok := findFirstInstructionInLine(line)
    if !ok {
        t.Fatalf("expected ok=true")
    }
    if instr != "rename var" {
        t.Fatalf("instr got %q want %q", instr, "rename var")
    }
    // Removal preserves inner spacing; trailing right spaces trimmed only.
    if cleaned != "prefix  suffix" {
        t.Fatalf("cleaned got %q want %q", cleaned, "prefix  suffix")
    }
}

func TestFindFirstInstructionInLine_StrictSemicolon_TrailingSpacesTrimmed(t *testing.T) {
    line := "code;fix;   \t\t"
    instr, cleaned, ok := findFirstInstructionInLine(line)
    if !ok {
        t.Fatalf("expected ok=true")
    }
    if instr != "fix" {
        t.Fatalf("instr got %q want %q", instr, "fix")
    }
    if cleaned != "code" {
        t.Fatalf("cleaned got %q want %q", cleaned, "code")
    }
}

func TestFindFirstInstructionInLine_Semicolon_InvalidPatterns(t *testing.T) {
    cases := []string{
        "prefix ; bad; suffix",  // space after first ';' ⇒ invalid
        "prefix ;bad ; suffix",  // space before closing ';' ⇒ invalid
        "prefix ; ; suffix",     // empty inner ⇒ invalid
    }
    for _, line := range cases {
        if instr, _, ok := findFirstInstructionInLine(line); ok && instr != "" {
            t.Fatalf("%q: expected no semicolon instruction; got instr=%q", line, instr)
        }
    }
}

func TestFindFirstInstructionInLine_CBlockComment(t *testing.T) {
    line := "foo /* update this part */ bar"
    instr, cleaned, ok := findFirstInstructionInLine(line)
    if !ok {
        t.Fatalf("expected ok=true")
    }
    if instr != "update this part" {
        t.Fatalf("instr got %q want %q", instr, "update this part")
    }
    if cleaned != "foo  bar" {
        t.Fatalf("cleaned got %q want %q", cleaned, "foo  bar")
    }
}

func TestFindFirstInstructionInLine_HTMLComment(t *testing.T) {
    line := "foo <!--  do x  --> bar"
    instr, cleaned, ok := findFirstInstructionInLine(line)
    if !ok {
        t.Fatalf("expected ok=true")
    }
    if instr != "do x" {
        t.Fatalf("instr got %q want %q", instr, "do x")
    }
    if cleaned != "foo  bar" {
        t.Fatalf("cleaned got %q want %q", cleaned, "foo  bar")
    }
}

func TestFindFirstInstructionInLine_SlashSlash(t *testing.T) {
    line := "val // do this change"
    instr, cleaned, ok := findFirstInstructionInLine(line)
    if !ok {
        t.Fatalf("expected ok=true")
    }
    if instr != "do this change" {
        t.Fatalf("instr got %q want %q", instr, "do this change")
    }
    if cleaned != "val" {
        t.Fatalf("cleaned got %q want %q", cleaned, "val")
    }
}

func TestFindFirstInstructionInLine_Hash(t *testing.T) {
    line := "val # do this"
    instr, cleaned, ok := findFirstInstructionInLine(line)
    if !ok {
        t.Fatalf("expected ok=true")
    }
    if instr != "do this" {
        t.Fatalf("instr got %q want %q", instr, "do this")
    }
    if cleaned != "val" {
        t.Fatalf("cleaned got %q want %q", cleaned, "val")
    }
}

func TestFindFirstInstructionInLine_DoubleDash(t *testing.T) {
    line := "SQL -- fix query"
    instr, cleaned, ok := findFirstInstructionInLine(line)
    if !ok {
        t.Fatalf("expected ok=true")
    }
    if instr != "fix query" {
        t.Fatalf("instr got %q want %q", instr, "fix query")
    }
    if cleaned != "SQL" {
        t.Fatalf("cleaned got %q want %q", cleaned, "SQL")
    }
}

func TestFindFirstInstructionInLine_EarliestWins_CommentOverSemicolon(t *testing.T) {
    line := "aa // comment ;not this; trailing"
    instr, cleaned, ok := findFirstInstructionInLine(line)
    if !ok {
        t.Fatalf("expected ok=true")
    }
    if instr != "comment ;not this; trailing" {
        t.Fatalf("instr got %q want %q", instr, "comment ;not this; trailing")
    }
    if cleaned != "aa" {
        t.Fatalf("cleaned got %q want %q", cleaned, "aa")
    }
}

func TestFindFirstInstructionInLine_EarliestWins_SemicolonOverComment(t *testing.T) {
    line := "aa ;short; // comment"
    instr, cleaned, ok := findFirstInstructionInLine(line)
    if !ok {
        t.Fatalf("expected ok=true")
    }
    if instr != "short" {
        t.Fatalf("instr got %q want %q", instr, "short")
    }
    // Only the earliest marker is removed; the later comment remains.
    if cleaned != "aa  // comment" {
        t.Fatalf("cleaned got %q want %q", cleaned, "aa  // comment")
    }
}

func TestFindStrictSemicolonTag_Various(t *testing.T) {
    // basic
    if text, l, r, ok := findStrictSemicolonTag("pre;do it;post"); !ok || text != "do it" || l != 3 || r != 10 {
        t.Fatalf("unexpected: ok=%v text=%q l=%d r=%d", ok, text, l, r)
    }
    // at start
    if text, l, r, ok := findStrictSemicolonTag(";x;"); !ok || text != "x" || l != 0 || r != 3 {
        t.Fatalf("unexpected at start: ok=%v text=%q l=%d r=%d", ok, text, l, r)
    }
    // double opening ';' should still allow a tag starting at the second ';'
    if text, _, _, ok := findStrictSemicolonTag("prefix ;;bad; suffix"); !ok || text != "bad" {
        t.Fatalf("unexpected double-open handling: ok=%v text=%q", ok, text)
    }
    // inner spaces directly after first ';' or before last ';' invalidate the tag
    if _, _, _, ok := findStrictSemicolonTag("a;  inner  ;b"); ok {
        t.Fatalf("expected invalid strict tag due to spaces at boundaries")
    }
}
