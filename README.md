# Hexai

![HexAI Small Logo](hexai-small.png)

Hexai, the AI LSP for the Helix editor and also a simple command line tool to interact with LLMs in general.

Hexai exposes a simple LLM provider interface. It supports OpenAI, GitHub Copilot, and a local Ollama server. Provider selection and models are configured via a JSON configuration file.

## Configuration

### Example configuration file

- Location: `$XDG_CONFIG_HOME/hexai/config.json` (usually `~/.config/hexai/config.json`)
- Example:

```
{
  "max_tokens": 4000,
  "context_mode": "always-full",
  "context_window_lines": 120,
  "max_context_tokens": 4000,
  "log_preview_limit": 100,
  "no_disk_io": true,
  "trigger_characters": [".", ":", "/", "_", ";", "?"],
  "provider": "ollama",
  "copilot_model": "gpt-4.1",
  "copilot_base_url": "https://api.githubcopilot.com",
  "openai_model": "gpt-4.1",
  "openai_base_url": "https://api.openai.com/v1",
  "ollama_model": "qwen2.5-coder:latest",
  "ollama_base_url": "http://localhost:11434"
}
```

* context_mode: minimal | window | file-on-new-func | always-full
* provider: openai | copilot | ollama
* openai_model, openai_base_url: OpenAI-only options
* copilot_model, copilot_base_url: Copilot-only options
* ollama_model, ollama_base_url: Ollama-only options

Ensure `OPENAI_API_KEY` or `COPILOT_API_KEY` is set in your environment according to your chosen provider.

### Selecting a provider

- Set `provider` in the config file to `openai`, `copilot`, or `ollama`.
- If omitted, Hexai defaults to `openai`.

### OpenAI configuration

- Required: `OPENAI_API_KEY` — provided via environment variable only.
- In config file:
  - `openai_model` — model name (default: `gpt-4.1`).
  - `openai_base_url` — API base (default: `https://api.openai.com/v1`).

### GitHub Copilot configuration

- Required: `COPILOT_API_KEY` — provided via environment variable only.
- In config file:
  - `copilot_model` — model name (default: `gpt-4.1`).
  - `copilot_base_url` — API base (default: `https://api.githubcopilot.com`).

### Ollama configuration (local)

- In config file:
  - `ollama_model` — model name/tag (default: `qwen2.5-coder:latest`).
  - `ollama_base_url` — base URL to Ollama (default: `http://localhost:11434`).

Notes:
- For Ollama, ensure the model is available locally (e.g., `ollama pull qwen2.5-coder:latest`).
- If you run Ollama in OpenAI‑compatible mode, you may alternatively use the
  OpenAI provider with `openai_base_url` in the config pointing to your local endpoint.

## Usage

### Hexai LSP Server

- Run LSP server over stdio:
  - `hexai-lsp`

- LSP flags (minimal):
  - `-version`: print the Hexai version and exit.
  - `-log`: path to log file (optional; default `/tmp/hexai-lsp.log`).

### Configure in Helix
 
In Helix'  `~/.config/helix/languages.toml`, configure for example the following:

```toml
[[language]]
name = "go"
auto-format= true
diagnostic-severity = "hint"
formatter = { command = "goimports" }
language-servers = [ "gopls", "golangci-lint-lsp", "hexai" ]

[language-server.hexai]
command = "hexai"
```

Note, that we have also configured other LSPs here (for Go, `gopls` and `golangci-lint-lsp`, along with `hexai` for AI completions - they aren't required for `hexai` to work, though)

## Inline triggers

Hexai LSP supports inline trigger tags you can type in your code to request an
action from the LLM and then clean up the tag automatically.

- `;some prompt here;`: Do what is written in `some prompt text here`, then remove just the prompt.
  - Strict form: no space after the first ``.
  - An optional single space immediately after the closing `;` is also removed.
- Spaced variants such as `; text ; spaced ;` are ignored.
- `some text here ;;some prompt;`

## Code actions

Hexai provides code actions that operate only on the current selection in Helix:

- Rewrite selection: Hexai looks for the first instruction inside the selection
  and rewrites the selection accordingly.
- Resolve diagnostics: With a selection active, Hexai gathers only diagnostics
  that overlap your selection and fixes them by editing only the selected code.
  Diagnostics outside the selection are not modified.

Instruction sources (first one found wins):

- Strict marker: `` (no space after first ``).
- Line comments: `// text`, `# text`, `-- text`.
- Single-line block comments: `/* text */`, `<!-- text -->`.

## Hexai CLI tool

- Run command-line tool (processes text via configured LLM):
  - `cat SOMEFILE.txt | hexai`
  - `hexai 'some prompt text here'`
  - `cat SOMEFILE.txt | hexai 'some prompt text here'` (stdin and arg are concatenated)

- Default style: concise answers.
  - If the prompt asks for commands, outputs only the commands with no commentary.
  - Add the word `explain` in your prompt to request a verbose explanation.
- Exit codes: `0` success, `1` provider/config error, `2` no input.

Examples:

```
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

