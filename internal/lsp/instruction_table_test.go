package lsp

import "testing"

func TestFindFirstInstructionInLine_Table(t *testing.T) {
    cases := []struct{
        name string
        line string
        instr string
    }{
        {"strict_inline_marker", ">do> trailing", "do"},
        {"c_block", "x /* add docs */ y", "add docs"},
        {"html_comment", "<!-- fix --> code", "fix"},
        {"slash_slash", "code // please refactor", "please refactor"},
        {"hash", "# summarize", "summarize"},
        {"double_dash", "-- rewrite quickly", "rewrite quickly"},
    }
    for _, c := range cases {
        instr, _, ok := findFirstInstructionInLine(c.line)
        if !ok || instr != c.instr {
            t.Fatalf("%s: got %q ok=%v", c.name, instr, ok)
        }
    }
}
