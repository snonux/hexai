package lsp

import (
	"fmt"
	"testing"
)

// fmtSscanContentLength parses a "Content-Length: N" header block.
func fmtSscanContentLength(hdr string, n *int) (int, error) {
	return fmt.Sscanf(hdr, "Content-Length: %d", n)
}

func TestByteOffsetToUTF16(t *testing.T) {
	s := "aé😀b"
	cases := map[int]int{0: 0, 1: 1, 3: 2, 7: 4, 8: 5, 99: 5}
	for in, want := range cases {
		if got := byteOffsetToUTF16(s, in); got != want {
			t.Fatalf("byteOffsetToUTF16(%d) = %d, want %d", in, got, want)
		}
		if in <= len(s) {
			if back := utf16OffsetToByteOffset(s, want); back != in {
				t.Fatalf("round trip %d -> %d -> %d", in, want, back)
			}
		}
	}
}

func TestPromptRemovalEditsForLine_UTF16(t *testing.T) {
	line := "ü := >!value>"
	edits := promptRemovalEditsForLine(line, 0, ">!", '>', '>')
	if len(edits) != 1 {
		t.Fatalf("expected one edit, got %+v", edits)
	}
	if edits[0].Range.Start.Character != 5 || edits[0].Range.End.Character != 13 {
		t.Fatalf("expected UTF-16 range 5..13, got %+v", edits[0].Range)
	}
}

func TestExtractRangeText_UTF16(t *testing.T) {
	d := &document{lines: []string{"ä := 1 // fix", "b"}}
	got := extractRangeText(d, Range{Start: Position{Line: 0, Character: 2}, End: Position{Line: 0, Character: 6}})
	if got != ":= 1" {
		t.Fatalf("got %q", got)
	}
	got = extractRangeText(d, Range{Start: Position{Line: 0, Character: 7}, End: Position{Line: 5, Character: 0}})
	if got != "// fix\nb" {
		t.Fatalf("got %q", got)
	}
}
