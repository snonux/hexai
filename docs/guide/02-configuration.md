# 2. Your first configuration

[← Installing](01-installing.md) · [Contents](index.md) · [Next: The command line →](03-cli.md)

Hexai reads a sectioned TOML file. Start with a small configuration and add
the parts you use; [config.toml.example](../../config.toml.example) lists
all supported sections and their comments.

![Opening the configuration in Helix, then sending a prompt](images/02-configuration.gif)

The animation points at a local response fixture. Use one of the real
provider configurations below on your own machine.

## Open the file

The default path is `~/.config/hexai/config.toml`, or
`$XDG_CONFIG_HOME/hexai/config.toml` when XDG_CONFIG_HOME is set.

```sh
export HEXAI_EDITOR=hx
hexai config
```

`HEXAI_EDITOR` takes precedence over `EDITOR`. Save your TOML and exit the
editor to return to the shell. You can also open a different file:

```sh
hexai --config ./my-config.toml config
hexai --config ./my-config.toml 'What is a slice in Go?'
```

## Use Ollama Cloud

The built-in provider is Ollama Cloud at `https://ollama.com`, with
`minimax-m3:cloud` for general use and `qwen3-coder:480b-cloud` for completions.
Provide your Ollama API key through the environment. The key variables are
`HEXAI_OLLAMA_API_KEY`, falling back to `OLLAMA_API_KEY`.

This makes the provider choice explicit:

```toml
[provider]
name = "ollama"

[ollama]
base_url = "https://ollama.com"
model = "minimax-m3:cloud"
temperature = 0.2
```

Pick models available to your account. The model names here describe the
repository's defaults rather than a promise about a provider's model catalog.

## Use a local Ollama server

With an Ollama server running and a coding model installed, set both the
general model and the completion model. Use the exact tag from your local
`ollama list`; substitute it for `your-local-model` in both places:

```toml
[provider]
name = "ollama"

[ollama]
base_url = "http://localhost:11434"
model = "your-local-model"

[models.completion]
provider = "ollama"
model = "your-local-model"
temperature = 0.0
```

Localhost Ollama needs no API key. Setting the completion entry matters:
otherwise the built-in completion model remains the cloud model.

## Hosted providers

| Provider name | Config section | Preferred key variable | Fallback key variable |
|---------------|----------------|------------------------|-----------------------|
| `ollama` | `[ollama]` | `HEXAI_OLLAMA_API_KEY` | `OLLAMA_API_KEY` |
| `openai` | `[openai]` | `HEXAI_OPENAI_API_KEY` | `OPENAI_API_KEY` |
| `openrouter` | `[openrouter]` | `HEXAI_OPENROUTER_API_KEY` | `OPENROUTER_API_KEY` |
| `anthropic` | `[anthropic]` | `HEXAI_ANTHROPIC_API_KEY` | `ANTHROPIC_API_KEY` |

For example, after making your OpenAI key available in the environment:

```toml
[provider]
name = "openai"

[openai]
model = "gpt-4.1"
base_url = "https://api.openai.com/v1"

[models.completion]
provider = "openai"
model = "gpt-4.1"
temperature = 0.0
```

The provider name selects the implementation; `base_url` selects its endpoint.
The same pattern applies to `[openrouter]` and `[anthropic]`; copy their
section from the example file and choose a model supported by your account.
Keep credentials in environment variables rather than project TOML.

## Share project settings

Create `.hexaiconfig.toml` at the git repository root to override only the
settings that differ for that project:

```toml
[general]
context_mode = "window"
context_window_lines = 60

[completion]
completion_debounce_ms = 1000
```

Hexai searches upwards from its working directory for `.git`, including
worktree `.git` files. Start Helix from the intended project so its server
finds that root. The merge order is:

1. Built-in defaults.
2. Global TOML, or the file selected with `--config`.
3. Project `.hexaiconfig.toml`.
4. Supported environment overrides.

`--config` changes the global-file layer; project settings and environment
overrides still apply. `hexai`, `hexai-lsp-server`, and `hexai-tmux-action`
accept `--config`. `ask` manages Taskwarrior settings separately.

## Environment overrides

Use environment overrides for a single session or command:

```sh
HEXAI_CONTEXT_MODE=window HEXAI_CONTEXT_WINDOW_LINES=40 hexai 'Explain slices'
HEXAI_PROVIDER_CLI=openai HEXAI_MODEL_CLI=gpt-4.1 hexai 'Explain slices'
```

Common variables include `HEXAI_PROVIDER`, `HEXAI_MAX_TOKENS`,
`HEXAI_MAX_CONTEXT_TOKENS`, `HEXAI_REQUEST_TIMEOUT` (seconds),
`HEXAI_CODING_TEMPERATURE`, `HEXAI_COMPLETION_DEBOUNCE_MS`,
`HEXAI_COMPLETION_THROTTLE_MS`, and `HEXAI_LOG_PREVIEW_LIMIT`.
Provider model, URL, and temperature use names such as `HEXAI_OPENAI_MODEL`,
`HEXAI_OPENAI_BASE_URL`, and `HEXAI_OPENAI_TEMPERATURE`.

The supported overrides are explicit; do not assume every TOML key has an
environment equivalent. [Models and providers](04-models.md) covers the
per-feature model and provider variables. After editing a running LSP's
configuration, use [reload or restart](06-chat-inline.md#slash-commands).
