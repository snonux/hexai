# The Hexai guide

A book about using Hexai, from your first command to an editor that completes
code, answers questions, and changes a selection for you. Each chapter explains
the feature, gives you something to try, and shows it running in a terminal.
Read in order to set everything up, or jump to the tool you need.

![Two models answering the same question](images/04-models.gif)

## Chapters

| # | Chapter | What you will do |
|---|---------|------------------|
| 1 | [Installing](01-installing.md) | Install the four binaries, Helix, tmux, and Taskwarrior; build from source |
| 2 | [Your first configuration](02-configuration.md) | Choose local or cloud inference; edit settings and override them per project |
| 3 | [The command line](03-cli.md) | Ask questions, pipe files, open your editor, cache replies, and simulate streaming |
| 4 | [Models and providers](04-models.md) | Choose a model per feature, compare providers, and configure fallback |
| 5 | [Completions in Helix](05-helix.md) | Register the LSP server, request completions, and control context and ignored files |
| 6 | [Chat and inline prompts](06-chat-inline.md) | Ask follow-up questions inside a file, replace prompt tags, and reload settings |
| 7 | [Code actions](07-actions.md) | Rewrite, document, fix diagnostics, generate Go tests, and use the tmux action menu |
| 8 | [Your own actions and prompts](08-custom-actions.md) | Add reusable actions, replace the menu, and customize prompt templates |
| 9 | [Tmux status](09-tmux.md) | Show live requests and usage statistics; set colors and width |
| 10 | [Project tasks with ask](10-tasks.md) | Scope tasks, manage dependencies and dates, watch progress, and enable Fish completion |
| 11 | [Files and troubleshooting](11-troubleshooting.md) | Find logs and caches, diagnose failures, and run development checks |

## Following along

Examples use a small Go function, `Add`, and a git repository called
`hexai-demo`. The CLI and task chapters work without an editor. Code completion,
chat, and actions need a configured provider; `ask` needs Taskwarrior but no LLM.
Shell examples use Bash unless a block says `fish`, `toml`, or `tmux`.

The recordings run the **real Hexai binaries**, Helix, tmux, and Taskwarrior.
LLM replies come from a deterministic **local HTTP fixture**, so they demonstrate
the interaction without requiring credentials or making paid API calls. The
`demo-fast` and `demo-careful` names in the animations are fixture names, not
models to install. Real provider replies, timing, versions, and task aliases
will differ. The TPS simulation in chapter 3 is Hexai's own offline mode.

Every chapter includes commands and explanations so you can follow it without
playing the animation. Click a GIF to open it if your viewer pauses animations.

The [commented configuration file](../../config.toml.example) is the complete
option reference. This guide covers the maintained tools; the experimental
MCP server keeps its [separate archival documentation](../mcp-setup.md).

## Recording the guide again

The [recording sources](recordings/readme.md) include VHS tapes, a local
response fixture, and a runner that uses temporary configuration, cache,
state, and Taskwarrior paths. From the repository root:

```sh
bash scripts/record-guide.sh
# Or render just one chapter:
bash scripts/record-guide.sh 06-chat-inline
```

Next: [Installing →](01-installing.md)
