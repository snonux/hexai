package hexaiaction

import (
    "bufio"
    "io"
    "strings"

    "codeberg.org/snonux/hexai/internal/textutil"
)

// ParseInput splits raw stdin into optional diagnostics and selection/code.
// Format:
//
//	Diagnostics:\n
//	<one per line>\n
//	<blank line> (optional)\n
//	<rest is selection/code>
//
// If the header is absent, the entire input is treated as selection.
func ParseInput(r io.Reader) (InputParts, error) {
	b, err := io.ReadAll(bufio.NewReader(r))
	if err != nil {
		return InputParts{}, err
	}
	raw := strings.TrimSpace(string(b))
	if raw == "" {
		return InputParts{Selection: ""}, nil
	}
	lines := strings.Split(raw, "\n")
	// find a case-insensitive line equal to "diagnostics:"
	diagsIdx := -1
	for i, ln := range lines {
		t := strings.TrimSpace(strings.ToLower(ln))
		if t == "diagnostics:" {
			diagsIdx = i
			break
		}
	}
	if diagsIdx < 0 {
		return InputParts{Selection: raw}, nil
	}
	// collect diagnostics until a blank line or EOF
	diags := []string{}
	i := diagsIdx + 1
	for ; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			i++
			break
		}
		diags = append(diags, t)
	}
	sel := strings.Join(lines[i:], "\n")
	sel = strings.TrimSpace(sel)
	return InputParts{Selection: sel, Diagnostics: diags}, nil
}

// ExtractInstruction mirrors the LSP instructionFromSelection behavior (subset),
// scanning the first line for an instruction marker and removing it from the selection.
func ExtractInstruction(sel string) (string, string) { return textutil.InstructionFromSelection(sel) }

// findFirstInstructionInLine follows the same precedence as LSP:
// - ;text; (strict)
// - /* text */ (single-line)
// - <!-- text --> (single-line)
// - // text
// - # text
// - -- text
// helpers moved to textutil
