# Recording the guide

The `.tape` files record the real CLI, Helix, tmux popups, Taskwarrior, and the experimental MCP server.
`fixture.py` serves deterministic OpenAI-compatible responses on a dynamically
allocated localhost port. It is a recording fixture, not an LLM or production
provider. The guide labels that distinction beside the animations.

## Requirements

Install Go matching the repository's `go.mod`, Python 3.11 or newer, VHS,
ffmpeg, ttyd, a Chromium browser usable by VHS, Helix (`hx`), tmux, Taskwarrior,
and Fish. No real API key, local LLM, or provider account is needed.

From the repository root:

```sh
bash scripts/record-guide.sh
# Render selected chapters only:
bash scripts/record-guide.sh 05-helix 06-chat-inline
```

The runner builds the four maintained binaries and the experimental MCP
server into a temporary directory.
Each tape gets a fresh sample git repository and temporary XDG config, cache,
state, and data directories. Taskwarrior uses a temporary TASKRC and TASKDATA;
the recording task aliases do not belong to your real task database. User
HOME is left as it is. Provider credentials and Hexai overrides are omitted
from the child environment, and all configured demo endpoints use localhost.
Temporary tmux servers use a dedicated socket within the sample directory.

Output GIFs are written to `docs/guide/images/`. `settings.tape` controls
the shared terminal dimensions, font, theme, and typing speed. Tape paths
are relative to the repository root; run the wrapper from any directory.

## What is captured

| Tape | Interaction |
|------|-------------|
| `01-installing` | Installed version and actual CLI help |
| `02-configuration` | `hexai config` opens Helix; a first CLI request |
| `03-cli` | Piped code, redirected stdout, and offline TPS simulation |
| `04-models` | Concurrent replies and a numbered cached selection |
| `04-fallback` | A primary 503 response and a successful sequential fallback |
| `05-helix` | A manual LSP completion accepted in Helix |
| `06-chat-inline` | An in-file chat reply and an inline replacement |
| `07-actions` | Real tmux popups for proofreading and documenting code |
| `07-lsp-actions` | A generated Go test file and a documented selection in Helix |
| `07-transformations` | Rewrite, Simplify, Go tests, and an editor-entered prompt |
| `08-custom-actions` | A project-defined custom main-menu action |
| `09-tmux` | Usage updates in tmux's actual status line |
| `10-tasks` | An isolated create/start/info/done lifecycle and Fish completion |
| `10-task-scope` | Directory scopes, dependencies, and unblocking |
| `11-troubleshooting` | An invalid rate, its failure status, and recovery |
| `12-mcp` | Real MCP discovery, create/get/update/delete, backups, and a missing-prompt error |

`demo-fast` and `demo-careful` are fixture model identifiers. Responses are
selected from the submitted prompt and streamed through the real HTTP client.
Timing shows the demonstration pacing, not real model latency. Installation
is documented as commands; the first tape inspects built tools rather than
performing a network installation during capture.

## Updating an interaction

Change the relevant tape and, if needed, its sample files in `record.py` or
its canned responses in `fixture.py`. Re-render that tape, inspect the GIF
(including intermediate editor/popup frames), and update the chapter text
if the interaction changed. Do not replace captured UI frames with drawings
or present fixture output as a measured model result.

The MCP recording uses `mcp-demo.py` to exchange real JSON-RPC messages over
stdio. It needs no LLM or response fixture, disables Markdown command export,
and keeps prompt data and logs in its temporary sample repository. The helper
checks matching response IDs and exits nonzero on protocol or tool errors.
