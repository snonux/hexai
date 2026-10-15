Output docs/guide/images/08-custom-actions.gif
Source docs/guide/recordings/settings.tape
Type "cat custom.toml | tail -14"
Enter
Sleep 3s
Hide
Type "cp custom.toml .hexaiconfig.toml"
Enter
Type `tmux -S "$HEXAI_DEMO/tmux.sock" -f tmux.conf new-session`
Enter
Sleep 1s
Show
Type "cat selection.go | hexai-tmux-action"
Enter
Sleep 3s
Type "e"
Sleep 3s
Type "exit"
Enter
