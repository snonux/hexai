# Helix + tmux Quickstart

This guide gets you from zero to editing with Hexai in Helix, with tmux showing live LLM status.

## 1) Install

- Install Mage (optional for build tasks): `go install github.com/magefile/mage@latest`
- Install binaries directly:
  - CLI: `go install codeberg.org/snonux/hexai/cmd/hexai@latest`
  - LSP: `go install codeberg.org/snonux/hexai/cmd/hexai-lsp@latest`
  - TUI: `go install codeberg.org/snonux/hexai/cmd/hexai-tmux-action@latest`

Ensure `~/go/bin` is on your `PATH`.

## 2) Configure Helix

In `~/.config/helix/languages.toml`:

```toml
[[language]]
name = "go"
auto-format = true
language-servers = ["gopls", "hexai"]

[language-server.hexai]
command = "hexai-lsp"
```

Optional keybindings in `~/.config/helix/config.toml` to run code actions on the selection:

```toml
[keys.select]
"A-a" = ":pipe hexai-tmux-action"

[keys.normal]
"A-a" = ["select_line", ":pipe hexai-tmux-action"]
```

## 3) Configure tmux status

Hexai can surface live progress in tmux's status line via a user option. Add this to your `~/.tmux.conf`:

```
set -g status-right '#{@hexai_status} #[fg=colour8]| %H:%M'
```

- Note: `colour8` is typically “bright black” (a dim grey) in many themes.
  If it’s low-contrast on your background, change it (e.g., `colour7` or `white`).

- CLI updates `@hexai_status` at start (⏳ provider:model) and on completion with compact stats (↑sent, ↓recv, rpm, reqs).
- LSP emits an initial heartbeat on client initialize and periodic compact stats (provider, model, rpm, reqs, bytes).
- The TUI action runner sets a ready heartbeat and a completion heartbeat with stats.
- Toggle with `HEXAI_TMUX_STATUS=0` to disable (enabled by default).

The status segment supports simple theming:

- Preset themes:
  - `HEXAI_TMUX_STATUS_THEME=white-on-purple` (white fg on purple/magenta bg)
  - `HEXAI_TMUX_STATUS_THEME=black-on-yellow` (black fg on yellow bg)
- Explicit colors: set any tmux color names or 256-color codes
  - `HEXAI_TMUX_STATUS_FG=white`
  - `HEXAI_TMUX_STATUS_BG=magenta` (or `colour5`, etc.)
- If the segment is truncated, widen it: `set -g status-right-length 120`

Add this to `~/.tmux.conf` and reload with `tmux source-file ~/.tmux.conf`:

```
set -g status-right '#{@hexai_status} #[fg=colour8]| %H:%M'
set -g status-right-length 120

# Optional: theme the Hexai status segment
set-environment -g HEXAI_TMUX_STATUS_THEME white-on-purple   # or black-on-yellow, white-on-blue
# Or explicit colors
# set-environment -g HEXAI_TMUX_STATUS_FG white
# set-environment -g HEXAI_TMUX_STATUS_BG magenta
```

## 4) Use it

- Start tmux, open Helix on a Go file.
- Try completions or inline prompts; or select code and press Alt-a for the action menu.
- Watch the right side of your tmux status for live LLM stats:
  - Start heartbeat: provider:model ⏳
  - Stats: ↑sent ↓recv rpm reqs

## 5) Troubleshooting

- No status? Verify: `tmux show -g -v @hexai_status` (should show text).
- Truncated? Increase width: `set -g status-right-length 120`.
- Disabled? Ensure `HEXAI_TMUX_STATUS` is not set to `0`.
- Wrong model? Rebuild/update binaries and restart Helix/LSP.

## Hexai Action (TUI) configuration

This is mostly useful when Helix runs in a [tmux](https://tmux.github.io/) session!

- Helix integration (recommended): bind a key to pipe the current selection to `hexai-tmux-action` and replace it with the output.
  - Example: `C-a = ":pipe hexai-tmux-action"`
- Default behavior:
  - Inline TUI when run in a real terminal (TTY).
  - When invoked via Helix `:pipe`, `hexai-tmux-action` opens a split pane to render the menu and returns the result on stdout for Helix to apply.
  - If no TTY and no tmux are available, it falls back to echoing the input.
- Flags:
  - `--infile`  Read input from the given file instead of stdin.
  - `--outfile` Write output to the given file instead of stdout (truncates/creates).
  - `--tmux-target` tmux target pane/window (advanced).
  - `--tmux-split v|h` split orientation (default: `v`).
  - `--tmux-percent N` split size percentage (default: `33`).
  - `--ui-child` internal; used by the parent process when spawning inside tmux.
