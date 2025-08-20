# Hexai usage and examples

This document describes how to run the LSP server, configure Helix, use in-editor chat,
inline triggers, code actions, and the CLI — with examples.

## LSP server

- Run over stdio: `hexai-lsp`
- Flags:
  - `-version`: print Hexai version and exit.
  - `-log`: path to log file (default `/tmp/hexai-lsp.log`).

### Configure in Helix

In `~/.config/helix/languages.toml`:

```toml
[[language]]
name = "go"
auto-format = true
diagnostic-severity = "hint"
formatter = { command = "goimports" }
language-servers = [ "gopls", "golangci-lint-lsp", "hexai" ]

[language-server.hexai]
command = "hexai-lsp"
```

Note: additional LSPs (`gopls`, `golangci-lint-lsp`) are optional; Hexai works without them.

## In-editor chat

Ask a question at the end of a line and receive the answer inline.

- End your question line with a trigger: `..`, `??`, `!!`, `::`, or `;;`.
- Hexai removes the trailing marker (last char for `..`/`??`/`!!`/`::`, both for `;;`).
- It inserts a blank line, then a reply line prefixed with `> `, then one extra newline so most
  editors place the cursor on a fresh blank line after the answer.
- If a `>` reply already exists below the question, Hexai won’t answer again.

Example:

```text
What is a slice in Go??

> A slice is a dynamically-sized, flexible view into the elements of an array. It references
> an underlying array and tracks length/capacity; most Go code uses slices instead of arrays.

```

Context: Hexai includes up to the three most recent Q/A pairs above the question when asking the LLM, so follow-ups remain on topic (e.g., “Are there many tourists?” after a location answer).

## Inline triggers

Hexai supports inline prompt tags you can type in code to request an action from the LLM and then auto-clean the tag. The strict semicolon form is supported:

- `;do something;` — Hexai uses the text between semicolons as the instruction and removes only the prompt. Strict form requires no space after the first `;` and no space before the closing `;`.

Spaced variants (e.g., `; spaced ;`) are ignored.

## Code actions

Operate on the current selection in Helix:

- Rewrite selection: finds the first instruction inside the selection and rewrites accordingly.
- Resolve diagnostics: gathers only diagnostics overlapping the selection and fixes them by editing the selected code; diagnostics outside the selection are not changed.

Instruction sources (first match wins):

- Strict marker: `;text;` (no space after first `;`).
- Line comments: `// text`, `# text`, `-- text`.
- Single-line block comments: `/* text */`, `<!-- text -->`.

## CLI usage

Process text via the configured LLM:

- `cat SOMEFILE.txt | hexai`
- `hexai 'some prompt text here'`
- `cat SOMEFILE.txt | hexai 'some prompt text here'` (stdin and arg are concatenated)

Defaults: concise answers. If the prompt asks for commands, Hexai outputs only commands. Add the word `explain` to request a verbose explanation. Exit codes: `0` success, `1` provider/config error, `2` no input.

### Examples

```sh
# From stdin only
cat SOMEFILE.txt | hexai

# From arg only
hexai 'summarize: list 3 bullets'

# From both (stdin first, then arg)
cat SOMEFILE.txt | hexai 'explain the tradeoffs'

# Commands-only output (no explanation)
hexai 'install ripgrep on macOS'

# Verbose explanation
hexai 'install ripgrep on macOS and explain'
```
