package lsp

import "testing"

func TestUTF16OffsetToByteOffset_ASCII(t *testing.T) {
	s := "hello world"
	if got := utf16OffsetToByteOffset(s, 5); got != 5 {
		t.Fatalf("expected 5, got %d", got)
	}
}

func TestUTF16OffsetToByteOffset_MultiByte(t *testing.T) {
	// "aé" — 'a' is 1 byte/1 UTF-16 unit, 'é' is 2 bytes/1 UTF-16 unit
	s := "aé"
	// UTF-16 offset 1 → byte 1 (after 'a')
	if got := utf16OffsetToByteOffset(s, 1); got != 1 {
		t.Fatalf("expected 1 after 'a', got %d", got)
	}
	// UTF-16 offset 2 → byte 3 (after 'é' which is 2 UTF-8 bytes)
	if got := utf16OffsetToByteOffset(s, 2); got != 3 {
		t.Fatalf("expected 3 after 'é', got %d", got)
	}
}

func TestUTF16OffsetToByteOffset_Emoji(t *testing.T) {
	// "a🎉b" — 'a' is 1/1, '🎉' is 4 bytes / 2 UTF-16 units, 'b' is 1/1
	s := "a🎉b"
	// UTF-16 offset 1 → byte 1 (after 'a')
	if got := utf16OffsetToByteOffset(s, 1); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	// UTF-16 offset 3 → byte 5 (after '🎉' which is 4 bytes, 2 UTF-16 units)
	if got := utf16OffsetToByteOffset(s, 3); got != 5 {
		t.Fatalf("expected 5 after emoji, got %d", got)
	}
	// UTF-16 offset 4 → byte 6 (after 'b')
	if got := utf16OffsetToByteOffset(s, 4); got != 6 {
		t.Fatalf("expected 6, got %d", got)
	}
}

func TestUTF16OffsetToByteOffset_BeyondEnd(t *testing.T) {
	s := "abc"
	if got := utf16OffsetToByteOffset(s, 10); got != 3 {
		t.Fatalf("expected len(s)=3 for offset beyond end, got %d", got)
	}
}

func TestUTF16OffsetToByteOffset_Empty(t *testing.T) {
	if got := utf16OffsetToByteOffset("", 0); got != 0 {
		t.Fatalf("expected 0 for empty string, got %d", got)
	}
}

func TestUTF16OffsetToByteOffset_Zero(t *testing.T) {
	if got := utf16OffsetToByteOffset("hello", 0); got != 0 {
		t.Fatalf("expected 0 for offset 0, got %d", got)
	}
}
