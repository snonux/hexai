Output docs/guide/images/07-actions.gif
Source docs/guide/recordings/settings.tape
Hide
Type `tmux -S "$HEXAI_DEMO/tmux.sock" -f tmux.conf new-session`
Enter
Sleep 1s
Show
Type "cat notes.txt | hexai-tmux-action"
Enter
Sleep 3s
Type "f"
Sleep 4s
Type "cat selection.go | hexai-tmux-action"
Enter
Sleep 2s
Type "c"
Sleep 3s
Type "exit"
Enter
