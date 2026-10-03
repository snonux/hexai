// Conversions between LSP positions (UTF-16 code units) and Go byte offsets.

package lsp

import "unicode/utf8"

// utf16OffsetToByteOffset converts an LSP UTF-16 code-unit offset to a byte
// offset within a Go (UTF-8) string. BMP characters (most code) are 1 UTF-16
// unit, while supplementary characters (e.g. emoji) are 2. Returns len(s)
// if the offset exceeds the string length.
func utf16OffsetToByteOffset(s string, utf16Offset int) int {
	byteIdx := 0
	units := 0
	for byteIdx < len(s) && units < utf16Offset {
		r, size := utf8.DecodeRuneInString(s[byteIdx:])
		byteIdx += size
		if r >= 0x10000 {
			units += 2 // surrogate pair in UTF-16
		} else {
			units++
		}
	}
	return byteIdx
}

// byteOffsetToUTF16 converts a byte offset within s to the LSP UTF-16
// code-unit offset. Offsets past the end of s are clamped to its length.
func byteOffsetToUTF16(s string, byteIdx int) int {
	if byteIdx > len(s) {
		byteIdx = len(s)
	}
	units := 0
	for i := 0; i < byteIdx; {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if r >= 0x10000 {
			units += 2
		} else {
			units++
		}
	}
	return units
}
