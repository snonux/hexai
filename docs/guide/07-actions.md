# 7. Code actions

[← Chat and inline prompts](06-chat-inline.md) · [Contents](index.md) · [Next: Your own actions →](08-custom-actions.md)

Use a code action when you have a selection to transform. The LSP exposes
actions through Helix's action picker; `hexai-tmux-action` gives a separate
menu with more built-in transformations and a free-form prompt editor.

![Proofreading text and documenting a function through tmux popups](images/07-actions.gif)

## Use the LSP action picker

In Helix, select code and press Space, then `a`. Available actions depend on
the file and selection:

| Action | What Hexai changes |
|--------|--------------------|
| Rewrite selection | Rewrites selected code according to an instruction found in it |
| Resolve diagnostics | Edits the selection to address diagnostics overlapping it |
| Implement unit test (Go) | Generates a `Test*` for the function under the cursor in a `.go` file |
| Document code | Adds documentation to the selected code |
| Simplify and improve | Simplifies the selected code while preserving behavior |
| Custom actions | Runs the user-defined actions from chapter 8 |

![Generating a sibling Go test file and documenting a selection](images/07-lsp-actions.gif)

For rewrite, put the instruction in the selection itself:

```go
// simplify this function
func Add(a, b int) int {
    result := a + b
    return result
}
```

Instruction extraction checks strict `;instruction;` markers, single-line
`/* instruction */` and `<!-- instruction -->` blocks, and line comments
such as `//`, `#`, and `--`. Keep the instruction near the start of the
selection. A strict semicolon marker has no space after the opening `;`.

Resolve diagnostics uses diagnostics that overlap the range, including those
published by another language server such as `gopls`. Diagnostics elsewhere
are outside the edit. Rewrites preserve leading selection indentation and
its trailing newline. A failed request produces an editor warning.

## Generate Go tests

Place the cursor in a Go function and choose the unit-test action. Hexai
creates the sibling `<file>_test.go` with a package and `testing` import if
needed, or appends test functions when the file already exists.

Review the generated cases and imports, then run `go test ./...` in your
project. Check error cases and boundaries as well as the example input; a
model can produce a plausible test with the wrong expectation.

## Use the tmux action menu

Start tmux, then pipe text to the runner:

```sh
printf 'The functoin retunrs teh sum of two intergers.\n' > notes.txt
cat notes.txt | hexai-tmux-action
```

It opens a tmux popup. Press `f` to proofread the text. The chosen result
returns to stdout after the popup closes. The animation then pipes the Add
function through `c`, Document code.

| Hotkey | Action |
|--------|--------|
| `r` | Rewrite selection using an embedded instruction |
| `i` | Simplify and improve |
| `c` | Document code |
| `t` | Generate Go unit tests |
| `f` | Fix typos, grammar, and clarity |
| `p` | Custom prompt, entered in your editor |
| `s` | Skip, returning the selection |
| `a` | Custom actions submenu, when configured (default submenu key) |

Arrow keys or `j`/`k` move through the menu, `g`/`G` jump to the first/last
entry, and Enter chooses. Escape or `q` skips. The prompt editor uses
`HEXAI_EDITOR` or `EDITOR` and a temporary Markdown file.

The next recording runs Rewrite, Simplify, Go tests, and an editor-entered
custom prompt in sequence:

![Four transformations through the real action runner](images/07-transformations.gif)

## Bind the menu in Helix

Run Helix inside tmux and add to `~/.config/helix/config.toml`:

```toml
[keys.select]
"A-a" = ":pipe hexai-tmux-action"

[keys.normal]
"A-a" = ["select_line", ":pipe hexai-tmux-action"]
```

Select text and press Alt-a to choose an action; Helix replaces the selection
with stdout. In normal mode this binding selects the current line first.
For generated tests, prefer the LSP action when you want the sibling test
file created automatically; the TUI returns text for the pipe to apply.

## Files, diagnostics, and popup options

The normal parent process reads **stdin** and manages temporary files for
the popup child. To process a file and save the result, use redirection to a
different destination:

```sh
hexai-tmux-action < input.go > output.go
```

`--infile` and `--outfile` belong to child mode (`--ui-child`); the popup
parent supplies them internally. They do not replace stdin/stdout in the
normal popup invocation. To run the same TUI directly in a terminal without
a popup, use its child path explicitly:

```sh
hexai-tmux-action --ui-child --infile input.go --outfile output.go
```

Advanced flags include `--tmux-target`, `--tmux-popup-width` (default `60%`),
and `--tmux-popup-height` (default `50%`). `--config` selects an alternate
configuration. When piping into the popup workflow, the parent's alternate
config path is not forwarded to the child; use the global or project file
for shared popup settings.

Input can include a `Diagnostics:` header, one diagnostic per line, a blank
line, then the code. That format provides diagnostic context to actions
that use it. Without the header, all input is the selection.
