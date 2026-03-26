# Fish completion for ask.
# Install as ~/.config/fish/completions/ask.fish or $XDG_CONFIG_HOME/fish/completions/ask.fish.

function __ask_needs_root_completion
    set -l tokens (commandline -opc)
    if test (count $tokens) -le 1
        return 0
    end
    for token in $tokens[2..-1]
        if not string match -qr '^-' -- $token
            return 1
        end
    end
    return 0
end

function __ask_in_dep_context
    set -l tokens (commandline -opc)
    if test (count $tokens) -lt 2
        return 1
    end
    set -l seen_dep 0
    for token in $tokens[2..-1]
        if string match -qr '^-' -- $token
            continue
        end
        if test $seen_dep -eq 0
            if test $token = dep
                set seen_dep 1
            else
                return 1
            end
        else
            return 1
        end
    end
    test $seen_dep -eq 1
end

complete -c ask -f
complete -c ask -s j -l json -d 'Emit JSON output'
complete -c ask -n '__ask_needs_root_completion' -a 'add' -d 'Create a new task'
complete -c ask -n '__ask_needs_root_completion' -a 'list' -d 'List active tasks'
complete -c ask -n '__ask_needs_root_completion' -a 'all' -d 'List all tasks'
complete -c ask -n '__ask_needs_root_completion' -a 'ready' -d 'List READY tasks'
complete -c ask -n '__ask_needs_root_completion' -a 'info' -d 'Show task details'
complete -c ask -n '__ask_needs_root_completion' -a 'annotate' -d 'Add an annotation'
complete -c ask -n '__ask_needs_root_completion' -a 'start' -d 'Start a task'
complete -c ask -n '__ask_needs_root_completion' -a 'stop' -d 'Stop a task'
complete -c ask -n '__ask_needs_root_completion' -a 'done' -d 'Mark a task complete'
complete -c ask -n '__ask_needs_root_completion' -a 'priority' -d 'Set priority'
complete -c ask -n '__ask_needs_root_completion' -a 'tag' -d 'Add or remove a tag'
complete -c ask -n '__ask_needs_root_completion' -a 'dep' -d 'Manage dependencies'
complete -c ask -n '__ask_needs_root_completion' -a 'urgency' -d 'List tasks sorted by urgency'
complete -c ask -n '__ask_needs_root_completion' -a 'modify' -d 'Modify task fields'
complete -c ask -n '__ask_needs_root_completion' -a 'denotate' -d 'Remove an annotation'
complete -c ask -n '__ask_needs_root_completion' -a 'delete' -d 'Delete a task'
complete -c ask -n '__ask_needs_root_completion' -a 'help' -d 'Show help'
complete -c ask -n '__ask_in_dep_context' -a 'add' -d 'Add a dependency'
complete -c ask -n '__ask_in_dep_context' -a 'rm' -d 'Remove a dependency'
complete -c ask -n '__ask_in_dep_context' -a 'list' -d 'List dependencies'
