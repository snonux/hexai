# Hexai

![HexAI Small Logo](hexai-small.png)

Hexai, the AI LSP for the Helix editor and also a simple command line tool to interact with LLMs in general.

At the moment this project is only in the proof of PoC phase.

## LLM provider

Hexai exposes a simple LLM provider interface. It supports OpenAI, GitHub Copilot, and a local Ollama server. Provider selection and models are configured via a JSON configuration file.

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

## CLI usage and configuration

- Run LSP server over stdio:
  - `hexai-lsp`

- LSP flags (minimal):
  - `-version`: print the Hexai version and exit.
  - `-log`: path to log file (optional; default `/tmp/hexai-lsp.log`).

- Run command-line tool (processes text via configured LLM):
  - `cat SOMEFILE.txt | hexai`
  - `hexai 'some prompt text here'`
  - `cat SOMEFILE.txt | hexai 'some prompt text here'` (stdin and arg are concatenated)

Notes for `hexai` (CLI):
- Prints LLM output to stdout.
- Prints provider/model immediately to stderr, and a summary to stderr at the end (time, input bytes, output bytes, provider/model).
- Default response style: short answers. If the prompt asks for commands, outputs only the commands with no explanation. Include the word `explain` anywhere in the prompt to request a verbose explanation.
- Streams output: when supported by the provider (OpenAI, Ollama), `hexai` streams tokens and prints them to stdout as they arrive. Copilot falls back to non-streaming.

### Hexai CLI behavior

- Inputs: reads from stdin, from a single argument, or both.
  - If both are provided, Hexai concatenates them with a blank line in between.
- Output routing:
  - Stdout: the LLM response only (no decorations).
  - Stderr: metadata and progress in grey on black (styled via ANSI):
    - Provider/model printed immediately when the request starts.
    - A final stats line on a new line: `done provider=… model=… time=… in_bytes=… out_bytes=…`.
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

Notes:
- Token estimation for truncation uses a simple 4 chars/token heuristic.
- Full-file context is only included by default when defining a new function to balance quality, latency, and cost.

- Location: `~/.config/hexai/config.json`
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
Minimal config (defaults to OpenAI):

```
{}
```

Ensure `OPENAI_API_KEY` or `COPILOT_API_KEY` is set in your environment according to your chosen provider.

## Inline triggers

Hexai supports inline trigger tags you can type in your code to request an
action from the LLM and then clean up the tag automatically.

- ``: Do what is written in `text`, then remove just the `` marker.
  - Strict form: no space after the first ``.
  - An optional single space immediately after the closing `;` is also removed.
  - Multiple markers per line are supported.
  - Example: `// TODO ` removes only the marker.
- Spaced variants such as `; text ; spaced ;` are ignored.

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

Notes:

- Only the earliest instruction in the selection is used; Hexai removes that marker/comment from the selection before sending it to the LLM.
- The action returns only the transformed code and replaces exactly the selected range.
