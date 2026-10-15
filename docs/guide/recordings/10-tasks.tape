Output docs/guide/images/10-tasks.gif
Source docs/guide/recordings/settings.tape
Type "ask add priority:H 'Document Add'"
Enter
Sleep 1s
Type "ask ready"
Enter
Sleep 2s
Type "# Use the alias printed above (captured here for the recording)."
Enter
Hide
Type `task_alias=$(ask ready | awk -F '|' 'NR==3 {gsub(/ /,"",$3); print $3}')`
Enter
Show
Type `ask start "$task_alias"`
Enter
Sleep 1s
Type "ask info"
Enter
Sleep 2s
Type `ask done "$task_alias"`
Enter
Sleep 1s
Type `fish -c 'ask fish | source; complete -C "ask dep "'`
Enter
Sleep 3s
