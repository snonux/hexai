# 9. Tmux status

[← Your own actions](08-custom-actions.md) · [Contents](index.md) · [Next: Project tasks →](10-tasks.md)

Hexai can put the active provider and usage statistics in tmux's status
line. You can see a request start and finish while keeping your editor open.
The CLI, LSP server, and action runner share usage statistics.

![Two CLI requests updating the tmux status line](images/09-tmux.gif)

## Add a status segment

Merge these lines into `~/.tmux.conf`, then reload it:

```tmux
set -g status-right '#{@hexai_status} #[fg=colour8]| %H:%M'
set -g status-right-length 120
```

```sh
tmux source-file ~/.tmux.conf
```

The setting replaces your current right-hand status format. If you already
have segments there, add `#{@hexai_status}` to that format instead.
Choose a brighter separator color if `colour8` is hard to see in your theme.

Start a fresh prompt inside tmux:

```sh
hexai 'What is a slice in Go?'
```

The CLI publishes a starting indicator with `provider:model`, then a compact
completion summary. The LSP publishes a heartbeat at initialization and
periodic statistics. The action runner publishes ready and completion updates.
Status updates are enabled by default; set `HEXAI_TMUX_STATUS=0` to turn them
off for a process. Outside tmux there is no status bar to update.

## Read the counters

The segment reports sent/received bytes (`↑` and `↓`), request count, and
requests per minute. The global aggregate is marked `Σ`, followed by its
sliding window, such as `Σ@1h` or `Σ@45m`. These counters describe usage;
they are not billing totals or exact token counts.

Configure the window in Hexai's TOML:

```toml
[stats]
window_minutes = 60
```

The default is 60 minutes, with a supported range of 1–1440 minutes. Stats
are stored in `stats.json` under the [cache directory](11-troubleshooting.md#file-locations)
and protected by a lock so the tools can share them.

## Choose colors and limit width

Set environment variables before starting Hexai, or add them to tmux's
session environment for future processes:

```tmux
set-environment -g HEXAI_TMUX_STATUS_THEME white-on-purple
# Alternative preset: black-on-yellow
# Explicit colors override the preset:
# set-environment -g HEXAI_TMUX_STATUS_FG white
# set-environment -g HEXAI_TMUX_STATUS_BG magenta
```

Use tmux color names or 256-color values such as `colour5`. For a small
terminal, these settings reduce the segment:

```tmux
set-environment -g HEXAI_TMUX_STATUS_NARROW 1
set-environment -g HEXAI_TMUX_STATUS_MAXLEN 40
```

Narrow mode favors the global summary; a width limit can elide the per-model
tail. Restart the shell or editor processes that should inherit changed
environment variables. Widen `status-right-length` if tmux still cuts off
the segment before Hexai's own width limit.
