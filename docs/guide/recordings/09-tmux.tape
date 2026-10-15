Output docs/guide/images/09-tmux.gif
Source docs/guide/recordings/settings.tape
Hide
Type "export HEXAI_TMUX_STATUS=1"
Enter
Type `tmux -S "$HEXAI_DEMO/tmux.sock" -f tmux.conf new-session`
Enter
Sleep 1s
Show
Type "hexai 'What is a slice in Go?'"
Enter
Sleep 4s
Type "hexai 'What does Add do?'"
Enter
Sleep 4s
Type "exit"
Enter
