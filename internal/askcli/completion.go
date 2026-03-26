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

func FishCompletion() string {
	var b strings.Builder
	b.WriteString("# Fish completion for ask.\n")
	b.WriteString("# Install as ~/.config/fish/completions/ask.fish or")
	b.WriteString(" $XDG_CONFIG_HOME/fish/completions/ask.fish.\n\n")
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
	b.WriteString("complete -c ask -f\n")
	b.WriteString("complete -c ask -s j -l json -d 'Emit JSON output'\n")
	for _, item := range askRootCompletionItems {
		writeFishCompletionLine(&b, "__ask_needs_root_completion", item)
	}
	for _, item := range askDepCompletionItems {
		writeFishCompletionLine(&b, "__ask_in_dep_context", item)
	}
	return b.String()
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
