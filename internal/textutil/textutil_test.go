package textutil

import (
	"regexp"
	"strings"
	"testing"
)

func TestRenderTemplate_Basic(t *testing.T) {
	out := RenderTemplate("Hello, {{name}}!", map[string]string{"name": "Hex"})
	if out != "Hello, Hex!" {
		t.Fatalf("render failed: %q", out)
	}
	// No vars
	if RenderTemplate("x", nil) != "x" {
		t.Fatal("nil vars changed output")
	}
}

func TestStripCodeFences_Variants(t *testing.T) {
	cases := []struct{ in, want string }{
		{"```\ncode\n```", "code"},
		{"```go\npackage x\n```", "package x"},
		{"no fences", "no fences"},
		{"\n\n```\ntrim\n```\n", "trim"},
	}
	for _, c := range cases {
		if got := StripCodeFences(c.in); got != c.want {
			t.Fatalf("strip mismatch: %q != %q", got, c.want)
		}
	}
}

func TestInstructionFromSelection_Markers(t *testing.T) {
	inputs := []string{
		";do it;\ncode",
		"/* fix */\ncode",
		"<!-- doc -->\ncode",
		"// change\ncode",
		"# tweak\ncode",
		"-- op\ncode",
	}
	for _, in := range inputs {
		instr, cleaned := InstructionFromSelection(in)
		if strings.TrimSpace(instr) == "" {
			t.Fatalf("no instruction for input: %q", in)
		}
		// cleaned should not contain the instruction token
		if strings.Contains(cleaned, instr) {
			// Allow coincidence only if separated differently; require not exact match on same line
			first := strings.Split(in, "\n")[0]
			if strings.Contains(first, instr) {
				t.Fatalf("instruction not removed: %q", cleaned)
			}
		}
	}
}

func TestFindFirstInstructionInLine_EarliestWins(t *testing.T) {
	// Both markers present, earliest should win (strict tag first)
	line := ";first;  // later"
	instr, cleaned, ok := FindFirstInstructionInLine(line)
	if !ok || instr != "first" {
		t.Fatalf("expected 'first', got %q ok=%v", instr, ok)
	}
	if strings.Contains(cleaned, instr) {
		t.Fatalf("expected cleaned line to remove instr: %q", cleaned)
	}
}

func TestFindStrictInlineTag(t *testing.T) {
	if txt, l, r, ok := FindStrictInlineTag("pre;do;post"); !ok || txt != "do" || l != 3 || r != 7 {
		t.Fatalf("strict tag parse failed: %q %d %d %v", txt, l, r, ok)
	}
	if _, _, _, ok := FindStrictInlineTag("; spaced ;"); ok {
		t.Fatalf("should reject spaced strict tag")
	}
}

// optional: ensure no ANSI codes appear in plain helpers
func TestNoANSIInHelpers(t *testing.T) {
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	if ansi.MatchString(RenderTemplate("x", nil)) {
		t.Fatalf("unexpected ansi in RenderTemplate")
	}
	if ansi.MatchString(StripCodeFences("x")) {
		t.Fatalf("unexpected ansi in StripCodeFences")
	}
}
