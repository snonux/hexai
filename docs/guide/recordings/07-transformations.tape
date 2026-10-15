Output docs/guide/images/07-transformations.gif
Source docs/guide/recordings/settings.tape
Hide
Type `tmux -S "$HEXAI_DEMO/tmux.sock" -f tmux.conf new-session`
Enter
Sleep 1s
Show
Type "cat rewrite.go | hexai-tmux-action"
Enter
Sleep 2s
Type "r"
Sleep 2s
Type "cat selection.go | hexai-tmux-action"
Enter
Sleep 2s
Type "i"
Sleep 2s
Type "cat selection.go | hexai-tmux-action"
Enter
Sleep 2s
Type "t"
Sleep 3s
Type "cat selection.go | hexai-tmux-action"
Enter
Sleep 2s
Type "p"
Sleep 2s
Type "iSimplify the selection. Return only code."
Escape
Type ":write-quit"
Enter
Sleep 3s
Type "exit"
Enter
