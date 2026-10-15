Output docs/guide/images/10-task-scope.gif
Source docs/guide/recordings/settings.tape
Hide
Type `first=$(ask add priority:H 'Write the guide'); first=${first##* }`
Enter
Show
Type "mkdir docs"
Enter
Type "cd docs"
Enter
Type "ask add 'Check chapter links'"
Enter
Type "ask list"
Enter
Sleep 2s
Type "cd .."
Enter
Type "ask list"
Enter
Sleep 2s
Type `ask add depends:"$first" 'Review the guide'`
Enter
Type "ask list +BLOCKED"
Enter
Sleep 2s
Type `ask done "$first"`
Enter
Type "ask ready"
Enter
Sleep 3s
