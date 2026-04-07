# Hexai usage and examples

This document describes how to run the LSP server, configure Helix, use in-editor chat,
inline triggers, code actions, and the CLI — with examples.

## LSP server

- Run over stdio: `hexai-lsp-server`
- Flags:
  - `-version`: print Hexai version and exit.
  - `-log`: path to log file (default `/tmp/hexai-lsp-server.log`).

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
command = "hexai-lsp-server"
```

Note: additional LSPs (`gopls`, `golangci-lint-lsp`) are optional; Hexai works without them.

## In-editor chat

Ask a question at the end of a line and receive the answer inline.

- End your question line with a trigger: `?>`, `!>`, or `:>`.
- Hexai removes only the trailing `>` from the question line (and keeps your trailing punctuation). Inline code-completion triggers now use `>!text>` (inline) or `>>!text>` (line-replace).
- It inserts a blank line, then a reply line prefixed with `> `, then one extra newline so most
  editors place the cursor on a fresh blank line after the answer.
- If a `>` reply already exists below the question, Hexai won’t answer again.

Example:

```text
What is a slice in Go?>

> A slice is a dynamically-sized, flexible view into the elements of an array. It references
> an underlying array and tracks length/capacity; most Go code uses slices instead of arrays.

```

Context: Hexai includes up to the three most recent Q/A pairs above the question when asking the LLM, so follow-ups remain on topic (e.g., “Are there many tourists?” after a location answer).

## Inline triggers

Hexai supports inline prompt tags you can type in code to request an action from the LLM and then auto-clean the tag. The new `>!`-based forms are:

- `>!do something>` — uses the text between markers as the instruction and removes only the prompt. Strict form requires no space after `>!` and no space before the closing `>`.
- `>>!do something>` — same as above, but replaces the entire current line with the completion.

Spaced variants (e.g., `> spaced >`) are ignored.

## Code actions

Operate on the current selection in Helix:

- Rewrite selection: finds the first instruction inside the selection and rewrites accordingly.
- Resolve diagnostics: gathers only diagnostics overlapping the selection and fixes them by editing the selected code; diagnostics outside the selection are not changed.
- Implement unit test (Go): when editing a `.go` file, adds a code action to generate a unit test for the function under the cursor. If `<file>_test.go` exists, appends a new `Test*`; otherwise creates the test file with `package` and `import "testing"`.
- Document code: adds idiomatic documentation comments to the selected code, preserving behavior and returning only the documented code.

Instruction sources (first match wins):

- Strict marker: `;text;` (no space after first `;`).
- Line comments: `// text`, `# text`, `-- text`.
- Single-line block comments: `/* text */`, `<!-- text -->`.

## CLI usage

Process text via the configured LLM:

- `cat SOMEFILE.txt | hexai`
- `hexai 'some prompt text here'`
- `cat SOMEFILE.txt | hexai 'some prompt text here'` (stdin and arg are concatenated)
- `hexai --tps-simulation 12-18` to simulate model output speed without calling a provider

Defaults: concise answers. If the prompt asks for commands, Hexai outputs only commands. Add the word `explain` to request a verbose explanation. Exit codes: `0` success, `1` provider/config error, `2` no input`.

Provider/model headers and run summaries are written to `stderr`, so `stdout` stays usable in pipes.

Successful CLI responses are cached for 24 hours under Hexai's cache directory. The cache key includes the input text, provider, resolved model, and effective CLI prompt text, so prompt/config changes automatically invalidate old entries.

To rerun a multi-provider prompt and print only one response cleanly, use the existing numbered provider flags such as `-0`, `-1`, etc. That reuses the cached response for just that provider when available, which avoids the side-by-side layout on `stdout`.

`--tps-simulation` accepts either a fixed rate such as `20` or a range such as `12-18`. It streams positional arguments, piped stdin, or built-in placeholder text when no input is provided, so you can preview perceived model latency without needing a real provider or local hardware.

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

# Warm the cache with two configured CLI providers, then print only the
# second provider's cached response on a rerun
hexai 'summarize this file'
hexai -1 'summarize this file'

# Simulate 12-18 tokens per second with placeholder text
hexai --tps-simulation 12-18

# Simulate how a file would feel when streamed back by a model
cat SOMEFILE.txt | hexai --tps-simulation 20
```

## Task management

`ask` is a task management CLI for the current git project. By default it auto-scopes to `project:<repo> +agent` so operations are confined to agent-managed project tasks.

Use `ask na <subcommand...>` or `ask no-agent <subcommand...>` to run the same subcommands against project tasks without the `+agent` tag. Those prefixes keep the project scope but replace the default tag filter with `-agent`.

`ask` never exposes Taskwarrior numeric task IDs. Human-facing output uses stable local alias IDs where practical, while `ask info` shows both the alias ID and the UUID. Commands that accept a task selector support either the alias ID or the UUID.

`ask` must be run inside a git repository so the project name can be derived from the repo root.

### Subcommands

| Subcommand | Description |
|---|---|
| `ask add "description"` | Create a new task and print `created task <alias-id>` |
| `ask add depends:<id\|uuid>,<id\|uuid> "description"` | Create task with inline dependencies |
| `ask add priority:H "description"` | Create task with priority |
| `ask add +tag "description"` | Create task with tag |
| `ask na add "description"` | Create a project task without the `+agent` tag |
| `ask list` | List pending tasks only (alias-ID table) |
| `ask na list` | List pending project tasks without the `+agent` tag |
| `ask all` | List all tasks including completed/deleted |
| `ask list +READY` | List only ready tasks |
| `ask list +BLOCKED` | List blocked tasks |
| `ask list +tag` | Filter by tag |
| `ask list started` | List started tasks |
| `ask list limit:N` | Limit results |
| `ask list sort:priority-,urgency-` | Sort by priority then urgency |
| `ask info [id\|uuid]` | Show task details, or the current started task if no selector is provided |
| `ask annotate <id\|uuid> "note"` | Add annotation |
| `ask start <id\|uuid>` | Start working on a task |
| `ask stop <id\|uuid>` | Stop work on a task |
| `ask done <id\|uuid>` | Mark task complete |
| `ask priority <id\|uuid> H\|M\|L` | Set priority |
| `ask tag <id\|uuid> +tag` | Add tag |
| `ask tag <id\|uuid> -tag` | Remove tag |
| `ask dep add <id\|uuid> <dep-id\|dep-uuid>` | Add dependency |
| `ask dep rm <id\|uuid> <dep-id\|dep-uuid>` | Remove dependency |
| `ask dep list <id\|uuid>` | List dependencies |
| `ask urgency` | List tasks by urgency |
| `ask modify <id\|uuid> <args...>` | General-purpose modify |
| `ask denotate <id\|uuid> "text"` | Remove annotation |
| `ask delete <id\|uuid>` | Delete a task |

### Examples

```sh
# Create a task
ask add priority:H "Implement new feature"

# Create a non-agent task
ask na add "Follow up manually"

# Create a task with dependencies
ask add +cli depends:0,1 "Implement dependent feature"

# List tasks
ask list +READY limit:5

# List non-agent tasks
ask no-agent list

# Show alias and UUID for a task
ask info 0

# Show a non-agent task
ask na info 0

# Start working
ask start 0

# Done
ask done 0
```

## Hexai Action (TUI)

`hexai-tmux-action` runs code actions over a selection or diagnostics+selection piped from stdin, or read from a file.

- Choose an action with arrow keys, `j/k`, `g/G`, Enter, or hotkeys `[s] [r] [c] [t]`.
  - Includes: Rewrite selection, Simplify and improve, Document code, Generate Go unit test(s), Skip.
  - “Custom prompt” (hotkey `[p]`) opens your editor (`$HEXAI_EDITOR` or `$EDITOR`) on a temporary `.md` file to write a free-form instruction.
- Output is written to stdout by default, or to a file via `--outfile`.

Input formats

- Rewrite: include an inline instruction near the top of the selection using one of:
  - `;do something;`
  - `/* do something */`
  - `<!-- do something -->`
  - `// do something` (or `#`, `--`)

Examples

```sh
# From stdin
cat input.go | hexai-tmux-action

# From file to file
hexai-tmux-action --infile input.go --outfile output.go

# Using shell redirection
hexai-tmux-action < input.go > output.go
```

### Helix keybinding example

Bind a key to pipe the current selection through the action runner and replace it in-place. In `~/.config/helix/config.toml`:

```toml
[keys.select]
# Alt-a runs the Hexai action menu on the selection
"A-a" = ":pipe hexai-tmux-action"

[keys.normal]
# Optional: run on the current line if no selection
"A-a" = ["select_line", ":pipe hexai-tmux-action"]
```

Tips:
- Ensure Helix runs inside tmux to see the status updates.
- You can also set a language-specific binding in `languages.toml` if preferred.

## Hexai Tmux Edit (Popup Editor)

`hexai-tmux-edit` opens your `$EDITOR` in a tmux popup for composing longer AI agent prompts. It captures existing prompt text from the target pane, pre-fills the editor, and sends the edited text back via `tmux send-keys`.

This is useful when working with AI CLI agents (Cursor, Amp, Aider, etc.) and you need to compose a longer, multi-line prompt with the comfort of your regular editor (spellcheck, search/replace, etc.).

OpenAI Codex CLI and Claude Code CLI are not built-in `hexai-tmux-edit` agents. Both support editing in an external editor natively via `Ctrl+G`.

### Supported agents

Built-in agent detection (auto-detected from pane content, checked in order):

1. **Cursor** -- detects box-drawing UI `│ →` or footer `/ commands · @ files`
   - Clears with: `End BSpace*200` (backspace method)
   - Prompt pattern: Extracts from last `│...│` box
2. **Amp** -- detects "amp" or "sourcegraph" in pane (TUI mode)
   - Clears with: `C-u` (Emacs/readline style)
   - Prompt pattern: Extracts from `│...│` box UI (similar to Cursor)
3. **Aider** -- detects "aider" in pane
   - Clears with: `C-u` (Emacs/readline style)
   - Prompt pattern: Shell-style `> prompt`

**Detection order matters**: Cursor is checked first to avoid false positives. For example, Cursor may display "Claude 4.5 Sonnet" as its model name, but Cursor's distinctive `│ →` box UI is matched first.

Additional agents can be added via `[tmux_edit.agents]` in config.toml without code changes.

### Tmux keybinding

Add to `~/.tmux.conf`:

```
bind e run-shell -b "cd '#{pane_current_path}' && hexai-tmux-edit --pane '#{pane_id}'"
```

The `#{pane_id}` is expanded by tmux to the active pane at keypress time, so the popup editor always knows which pane to send text back to.

### Flags

- `--config` path to config file (default: `$XDG_CONFIG_HOME/hexai/config.toml`)
- `--agent` explicit agent name (auto-detected if omitted)
- `--pane` tmux target pane ID (e.g. `%5`)

### Workflow

1. Press your tmux keybinding (e.g. `prefix + e`)
2. A tmux popup opens with your `$EDITOR`, pre-filled with any existing prompt text
3. Edit or compose your prompt
4. Save and close the editor
5. The edited text is sent to the agent's pane via `tmux send-keys`

If you keep the original text unchanged and append new text, only the appended text is sent. If you rewrite the prompt entirely, the full new text is sent. If you save an empty file or don't change anything, nothing is sent.

### Configuration

See `[tmux_edit]` in [config.toml.example](../config.toml.example) for all options, including custom popup dimensions and agent overrides.

### Slash commands

Type a slash command at the end of a chat line (for example `/? reload>`). Available commands:
- `/reload>` — reload configuration from disk (ignores environment overrides during the reload).
- `/disable>` — temporarily disable Hexai auto-completions; code actions and chat prompts continue to work.
- `/enable>` — re-enable Hexai auto-completions after a `/disable>`.
