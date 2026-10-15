# 5. Completions in Helix

[← Models and providers](04-models.md) · [Contents](index.md) · [Next: Chat and inline prompts →](06-chat-inline.md)

`hexai-lsp-server` speaks LSP over stdin and stdout. Helix starts it for you
and can run it alongside your usual language server. Keep `gopls` for Go
navigation and diagnostics; add Hexai for model suggestions and actions.

![Requesting and accepting an Add completion in Helix](images/05-helix.gif)

## Register the server

Add this to `~/.config/helix/languages.toml` (or the matching file below
`$XDG_CONFIG_HOME`). Merge the settings into any language entry you already
have:

```toml
[language-server.hexai]
command = "hexai-lsp-server"

[[language]]
name = "go"
language-servers = ["gopls", "hexai"]

[[language]]
name = "markdown"
language-servers = ["hexai"]
```

`gopls` must be installed separately if you list it. For a minimal setup, use
`language-servers = ["hexai"]`, as the recording does. Add Hexai to other
language entries in the same way; general completions and chat work beyond
Go, while the function-to-test action is Go specific.

The server accepts `--config /path/to/config.toml`, `--log /path/to/log`,
and `--version`. Pass server flags with Helix's `args` setting:

```toml
[language-server.hexai]
command = "hexai-lsp-server"
args = ["--config", "/absolute/path/to/config.toml"]
```

Use one server definition, choosing either the default file or explicit args.
Start Helix from your project root, then run `hx --health go` from the same
shell if the server is not found. Existing Helix sessions may need a language
server restart after you change `languages.toml`.

## Ask for a completion

Create `add.go`:

```go
package demo

func Add(a, b int) int {
    return
}
```

Place the cursor after `return` in insert mode and add a space. Press `Ctrl-x` to request a
completion, use `Ctrl-n`/`Ctrl-p` to choose a suggestion, and Enter to accept.
The animation turns off Helix's automatic completion to make the manual
request easy to see.

Helix normally requests completions while you type. Hexai recognizes
configured trigger characters; it can also suggest parameter-list contents
inside a function signature. With several configured completion entries,
suggestions carry provider/model labels.

## Control request frequency

```toml
[completion]
completion_debounce_ms = 800
completion_throttle_ms = 0
manual_invoke_min_prefix = 0
completion_wait_all = true

[triggers]
trigger_characters = [".", ":", "/", "_", " "]
```

Debounce waits for idle time before requesting. Throttle limits request
frequency; zero disables it. The manual prefix setting requires that many
identifier characters before a manual request. You can also disable automatic
completion through the [chat command](06-chat-inline.md#slash-commands)
without disabling chat and actions.

## Choose the context sent to the model

```toml
[general]
context_mode = "window"
context_window_lines = 60
max_context_tokens = 4000
max_tokens = 4000
request_timeout = 600
coding_temperature = 0.2
```

| Mode | Additional document context |
|------|-----------------------------|
| `minimal` | Only the request's immediate payload |
| `window` | A window around the cursor, controlled by `context_window_lines` |
| `file-on-new-func` | The full file when starting a new function |
| `always-full` | The full open file (the built-in default) |

`max_context_tokens` bounds additional context; `max_tokens` bounds requested
output. Context token counting is an estimate. Choose the mode to suit your
provider and file size. Hexai includes Go function context where applicable;
it is not a repository index.

## Ignore files

```toml
[ignore]
gitignore = true
extra_patterns = ["*.min.js", "vendor/**", "*.generated.go"]
lsp_notify_ignored = true
```

By default normal completions and code actions respect `.gitignore`.
Add patterns to skip those requests, and control whether ignored-file
completions explain the skip. Chat and inline prompts triggered by document
changes still run in ignored files; these patterns do not prevent sending
those prompts. Environment equivalents are `HEXAI_IGNORE_GITIGNORE`,
`HEXAI_IGNORE_EXTRA_PATTERNS` (comma-separated), and `HEXAI_IGNORE_LSP_NOTIFY`.

Next, [ask a question directly inside the file](06-chat-inline.md).
