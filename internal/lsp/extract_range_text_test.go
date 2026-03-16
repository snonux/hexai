package lsp

import "testing"

func TestExtractRangeText_NilDocument(t *testing.T) {
	got := extractRangeText(nil, Range{})
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtractRangeText_EmptyLines(t *testing.T) {
	d := &document{lines: []string{}}
	got := extractRangeText(d, Range{Start: Position{0, 0}, End: Position{0, 5}})
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtractRangeText_NegativeStartLine(t *testing.T) {
	d := &document{lines: []string{"hello"}}
	got := extractRangeText(d, Range{Start: Position{-1, 0}, End: Position{0, 3}})
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtractRangeText_NegativeEndLine(t *testing.T) {
	d := &document{lines: []string{"hello"}}
	got := extractRangeText(d, Range{Start: Position{0, 0}, End: Position{-1, 3}})
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtractRangeText_StartLineBeyondEnd(t *testing.T) {
	d := &document{lines: []string{"hello", "world"}}
	got := extractRangeText(d, Range{Start: Position{5, 0}, End: Position{6, 0}})
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtractRangeText_EndLineBeyondEnd(t *testing.T) {
	// End line is clamped to the last valid line.
	d := &document{lines: []string{"hello", "world"}}
	got := extractRangeText(d, Range{Start: Position{0, 0}, End: Position{10, 5}})
	if got != "hello\nworld" {
		t.Fatalf("expected clamped result, got %q", got)
	}
}

func TestExtractRangeText_StartAfterEnd(t *testing.T) {
	d := &document{lines: []string{"aaa", "bbb"}}
	got := extractRangeText(d, Range{Start: Position{1, 0}, End: Position{0, 2}})
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtractRangeText_SingleLine(t *testing.T) {
	d := &document{lines: []string{"hello world"}}
	got := extractRangeText(d, Range{Start: Position{0, 6}, End: Position{0, 11}})
	if got != "world" {
		t.Fatalf("expected %q, got %q", "world", got)
	}
}

func TestExtractRangeText_SingleLineNegativeChar(t *testing.T) {
	d := &document{lines: []string{"hello"}}
	got := extractRangeText(d, Range{Start: Position{0, -3}, End: Position{0, 3}})
	if got != "hel" {
		t.Fatalf("expected %q, got %q", "hel", got)
	}
}

func TestExtractRangeText_MultiLine(t *testing.T) {
	d := &document{lines: []string{"aaa", "bbb", "ccc"}}
	got := extractRangeText(d, Range{Start: Position{0, 1}, End: Position{2, 2}})
	if got != "aa\nbbb\ncc" {
		t.Fatalf("expected %q, got %q", "aa\nbbb\ncc", got)
	}
}
