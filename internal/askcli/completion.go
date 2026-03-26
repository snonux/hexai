package askcli

import (
	"strings"
)

type fishCompletionItem struct {
	name        string
	description string
}

var askRootCompletionItems = []fishCompletionItem{
	{name: "add", description: "Create a new task"},
	{name: "list", description: "List active tasks"},
	{name: "all", description: "List all tasks"},
	{name: "ready", description: "List READY tasks"},
	{name: "info", description: "Show task details"},
	{name: "annotate", description: "Add an annotation"},
	{name: "start", description: "Start a task"},
	{name: "stop", description: "Stop a task"},
	{name: "done", description: "Mark a task complete"},
	{name: "priority", description: "Set priority"},
	{name: "tag", description: "Add or remove a tag"},
	{name: "dep", description: "Manage dependencies"},
	{name: "urgency", description: "List tasks sorted by urgency"},
	{name: "modify", description: "Modify task fields"},
	{name: "denotate", description: "Remove an annotation"},
	{name: "delete", description: "Delete a task"},
	{name: "help", description: "Show help"},
}

var askDepCompletionItems = []fishCompletionItem{
	{name: "add", description: "Add a dependency"},
	{name: "rm", description: "Remove a dependency"},
	{name: "list", description: "List dependencies"},
}

var askUUIDCompletionItems = []fishCompletionItem{
	{name: "info", description: "Show task details"},
	{name: "annotate", description: "Add an annotation"},
	{name: "start", description: "Start a task"},
	{name: "stop", description: "Stop a task"},
	{name: "done", description: "Mark a task complete"},
	{name: "priority", description: "Set priority"},
	{name: "tag", description: "Add or remove a tag"},
	{name: "modify", description: "Modify task fields"},
	{name: "denotate", description: "Remove an annotation"},
	{name: "delete", description: "Delete a task"},
}

var askDepUUIDCompletionItems = []fishCompletionItem{
	{name: "add", description: "Add a dependency"},
	{name: "rm", description: "Remove a dependency"},
	{name: "list", description: "List dependencies"},
}

func FishCompletion() string {
	var b strings.Builder
	writeFishPreamble(&b)
	writeFishContextFunctions(&b)
	writeFishTaskUUIDFunction(&b)
	b.WriteString("complete -c ask -f\n")
	b.WriteString("complete -c ask -s j -l json -d 'Emit JSON output'\n")
	for _, item := range askRootCompletionItems {
		writeFishCompletionLine(&b, "__ask_needs_root_completion", item)
	}
	for _, item := range askDepCompletionItems {
		writeFishCompletionLine(&b, "__ask_in_dep_context", item)
	}
	for _, item := range askUUIDCompletionItems {
		writeFishUUIDCompletionLine(&b, "__ask_in_uuid_context", item.description)
	}
	for _, item := range askDepUUIDCompletionItems {
		writeFishUUIDCompletionLine(&b, "__ask_in_dep_uuid_context", item.description)
	}
	return b.String()
}

func writeFishPreamble(b *strings.Builder) {
	b.WriteString("# Fish completion for ask.\n")
	b.WriteString("# Install as ~/.config/fish/completions/ask.fish or")
	b.WriteString(" $XDG_CONFIG_HOME/fish/completions/ask.fish.\n\n")
}

func writeFishContextFunctions(b *strings.Builder) {
	writeFishNeedsRootCompletionFunction(b)
	writeFishDepContextFunction(b)
	writeFishUUIDContextFunction(b)
	writeFishDepUUIDContextFunction(b)
}

func writeFishNeedsRootCompletionFunction(b *strings.Builder) {
	b.WriteString("function __ask_needs_root_completion\n")
	b.WriteString("    set -l tokens (commandline -opc)\n")
	b.WriteString("    if test (count $tokens) -le 1\n")
	b.WriteString("        return 0\n")
	b.WriteString("    end\n")
	b.WriteString("    for token in $tokens[2..-1]\n")
	b.WriteString("        if not string match -qr '^-' -- $token\n")
	b.WriteString("            return 1\n")
	b.WriteString("        end\n")
	b.WriteString("    end\n")
	b.WriteString("    return 0\n")
	b.WriteString("end\n\n")
}

func writeFishDepContextFunction(b *strings.Builder) {
	b.WriteString("function __ask_in_dep_context\n")
	b.WriteString("    set -l tokens (commandline -opc)\n")
	b.WriteString("    if test (count $tokens) -lt 2\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    set -l seen_dep 0\n")
	b.WriteString("    for token in $tokens[2..-1]\n")
	b.WriteString("        if string match -qr '^-' -- $token\n")
	b.WriteString("            continue\n")
	b.WriteString("        end\n")
	b.WriteString("        if test $seen_dep -eq 0\n")
	b.WriteString("            if test $token = dep\n")
	b.WriteString("                set seen_dep 1\n")
	b.WriteString("            else\n")
	b.WriteString("                return 1\n")
	b.WriteString("            end\n")
	b.WriteString("        else\n")
	b.WriteString("            return 1\n")
	b.WriteString("        end\n")
	b.WriteString("    end\n")
	b.WriteString("    test $seen_dep -eq 1\n")
	b.WriteString("end\n\n")
}

func writeFishUUIDContextFunction(b *strings.Builder) {
	b.WriteString("function __ask_in_uuid_context\n")
	b.WriteString("    set -l tokens (commandline -opc)\n")
	b.WriteString("    set -l positional\n")
	b.WriteString("    for token in $tokens[2..-1]\n")
	b.WriteString("        if string match -qr '^-' -- $token\n")
	b.WriteString("            continue\n")
	b.WriteString("        end\n")
	b.WriteString("        set -a positional $token\n")
	b.WriteString("    end\n")
	b.WriteString("    if test (count $positional) -eq 0\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    if test (count $positional) -gt 2\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    switch $positional[1]\n")
	b.WriteString("        case info annotate start stop done priority tag modify denotate delete\n")
	b.WriteString("            return 0\n")
	b.WriteString("        case '*'\n")
	b.WriteString("            return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    return 1\n")
	b.WriteString("end\n\n")
}

func writeFishDepUUIDContextFunction(b *strings.Builder) {
	b.WriteString("function __ask_in_dep_uuid_context\n")
	b.WriteString("    set -l tokens (commandline -opc)\n")
	b.WriteString("    set -l positional\n")
	b.WriteString("    for token in $tokens[2..-1]\n")
	b.WriteString("        if string match -qr '^-' -- $token\n")
	b.WriteString("            continue\n")
	b.WriteString("        end\n")
	b.WriteString("        set -a positional $token\n")
	b.WriteString("    end\n")
	b.WriteString("    if test (count $positional) -lt 2\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    if test $positional[1] != dep\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    switch $positional[2]\n")
	b.WriteString("        case add rm\n")
	b.WriteString("            if test (count $positional) -le 4\n")
	b.WriteString("                return 0\n")
	b.WriteString("            end\n")
	b.WriteString("        case list\n")
	b.WriteString("            if test (count $positional) -le 3\n")
	b.WriteString("                return 0\n")
	b.WriteString("            end\n")
	b.WriteString("        case '*'\n")
	b.WriteString("            return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    return 1\n")
	b.WriteString("end\n\n")
}

func writeFishTaskUUIDFunction(b *strings.Builder) {
	b.WriteString("function __ask_task_uuids\n")
	b.WriteString("    command ask all --json 2>/dev/null | string match -r -a -g '\"uuid\":\"([^\"]+)\"'\n")
	b.WriteString("end\n\n")
}

func writeFishCompletionLine(b *strings.Builder, condition string, item fishCompletionItem) {
	b.WriteString("complete -c ask -n '")
	b.WriteString(condition)
	b.WriteString("' -a '")
	b.WriteString(item.name)
	b.WriteString("' -d '")
	b.WriteString(strings.ReplaceAll(item.description, "'", "\\'"))
	b.WriteString("'\n")
}

func writeFishUUIDCompletionLine(b *strings.Builder, condition, description string) {
	b.WriteString("complete -c ask -n '")
	b.WriteString(condition)
	b.WriteString("' -a '(__ask_task_uuids)' -d '")
	b.WriteString(strings.ReplaceAll(description, "'", "\\'"))
	b.WriteString("'\n")
}
