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

