# Hexai

![Hexai logo](hexai-small.png)

Hexai adds AI completions, code actions, and inline chat to the
[Helix editor](https://helix-editor.com). It also includes a command line
LLM client, a tmux action menu, and `ask`, a project task CLI built on
Taskwarrior. Other LSP editors may work, but Helix is the tested integration.

**[Read the Hexai guide](docs/guide/index.md)** — install the tools, choose
your providers, and work through every feature with animated terminal recordings.

![Comparing two models and selecting a cached answer](docs/guide/images/04-models.gif)

- Complete code with one model or compare suggestions from several.
- Ask questions inside a file, or use inline instructions to change code.
- Rewrite, document, fix diagnostics, and generate Go tests from a selection.
- Define your own actions and run them through Helix or a tmux popup.
- Send text through `hexai`, compare model responses, and reuse cached answers.
- Track project work with `ask`, stable task aliases, dependencies, and Fish completion.

Providers: Ollama (local or cloud), OpenAI, OpenRouter, and Anthropic.
The built-in default is Ollama Cloud; the guide also covers local setup.

## Install

With Go 1.26 or newer:

```sh
go install github.com/snonux/hexai/cmd/hexai@latest
go install github.com/snonux/hexai/cmd/hexai-lsp-server@latest
go install github.com/snonux/hexai/cmd/hexai-tmux-action@latest
go install github.com/snonux/hexai/cmd/ask@latest
```

Continue with [installation and prerequisites](docs/guide/01-installing.md)
and [your first configuration](docs/guide/02-configuration.md).
The complete commented settings reference is [config.toml.example](config.toml.example).

The experimental MCP server is outside this guide. Its
[existing reference](docs/mcp-setup.md) remains available for archival use.
