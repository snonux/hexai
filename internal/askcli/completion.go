package askcli

import (
	"strings"
)

type fishCompletionItem struct {
	name        string
	description string
}

var askSingleSelectorCompletionCommands = []string{
	"info",
	"annotate",
	"start",
	"stop",
	"done",
	"priority",
	"tag",
	"modify",
	"denotate",
	"delete",
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
	{name: "fish", description: "Emit Fish shell completion script"},
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

func fishSingleSelectorCompletionContext(positional []string) bool {
	if len(positional) != 1 {
		return false
	}

	for _, command := range askSingleSelectorCompletionCommands {
		if positional[0] == command {
			return true
		}
	}
	return false
}

func fishDepSelectorCompletionContext(positional []string) bool {
	if len(positional) < 2 || positional[0] != "dep" {
		return false
	}

	switch positional[1] {
	case "add", "rm":
		return len(positional) == 2 || len(positional) == 3
	case "list":
		return len(positional) == 2
	default:
		return false
	}
}

func fishAddDependencyModifierCompletionContext(positional []string, current string) bool {
	if len(positional) == 0 || positional[0] != "add" {
		return false
	}
	current = strings.TrimSpace(current)
	return current == "depends" || strings.HasPrefix(current, "depends:")
}

func FishCompletion() string {
	return FishCompletionFor("ask")
}

func FishCompletionFor(binaryPath string) string {
	var b strings.Builder
	writeFishPreamble(&b)
	writeFishContextFunctions(&b)
	writeFishTaskSelectorFunction(&b, binaryPath)
	writeFishAddDependencyModifierFunction(&b)
	b.WriteString("complete -c ask -f\n")
	b.WriteString("complete -c ask -s j -l json -d 'Emit JSON output'\n")
	for _, item := range askRootCompletionItems {
		writeFishCompletionLine(&b, "__ask_needs_root_completion", item)
	}
	for _, item := range askDepCompletionItems {
		writeFishCompletionLine(&b, "__ask_in_dep_context", item)
	}
	writeFishUUIDCompletionLine(&b, "__ask_in_uuid_context", "Task selector")
	writeFishUUIDCompletionLine(&b, "__ask_in_dep_uuid_context", "Task selector")
	writeFishFunctionCompletionLine(&b, "__ask_in_add_dep_modifier_context", "__ask_add_dependency_modifiers", "Task dependency")
	return b.String()
}

func writeFishPreamble(b *strings.Builder) {
	b.WriteString("# Fish completion for ask.\n")
	b.WriteString("# Source with: ask fish | source\n\n")
}

func writeFishContextFunctions(b *strings.Builder) {
	writeFishNeedsRootCompletionFunction(b)
	writeFishDepContextFunction(b)
	writeFishUUIDContextFunction(b)
	writeFishDepUUIDContextFunction(b)
	writeFishAddDependencyModifierContextFunction(b)
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
	b.WriteString("    if test (count $positional) -ne 1\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    switch $positional[1]\n")
	b.WriteString("        case ")
	b.WriteString(strings.Join(askSingleSelectorCompletionCommands, " "))
	b.WriteString("\n")
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
	b.WriteString("            if test (count $positional) -eq 2 -o (count $positional) -eq 3\n")
	b.WriteString("                return 0\n")
	b.WriteString("            end\n")
	b.WriteString("        case list\n")
	b.WriteString("            if test (count $positional) -eq 2\n")
	b.WriteString("                return 0\n")
	b.WriteString("            end\n")
	b.WriteString("        case '*'\n")
	b.WriteString("            return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    return 1\n")
	b.WriteString("end\n\n")
}

func writeFishAddDependencyModifierContextFunction(b *strings.Builder) {
	b.WriteString("function __ask_in_add_dep_modifier_context\n")
	b.WriteString("    set -l tokens (commandline -opc)\n")
	b.WriteString("    set -l positional\n")
	b.WriteString("    for token in $tokens[2..-1]\n")
	b.WriteString("        if string match -qr '^-' -- $token\n")
	b.WriteString("            continue\n")
	b.WriteString("        end\n")
	b.WriteString("        set -a positional $token\n")
	b.WriteString("    end\n")
	b.WriteString("    if test (count $positional) -lt 1\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    if test $positional[1] != add\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    set -l current (commandline -ct)\n")
	b.WriteString("    if test $current = depends\n")
	b.WriteString("        return 0\n")
	b.WriteString("    end\n")
	b.WriteString("    string match -qr '^depends:' -- $current\n")
	b.WriteString("end\n\n")
}

func writeFishTaskSelectorFunction(b *strings.Builder, binaryPath string) {
	b.WriteString("function __ask_task_selectors\n")
	b.WriteString("    set -l ask_bin ")
	b.WriteString(quoteFishString(binaryPath))
	b.WriteString("\n")
	b.WriteString("    set -l now (date +%s)\n")
	b.WriteString("    if set -q __ask_task_selector_cache_until; and test $__ask_task_selector_cache_until -ge $now\n")
	b.WriteString("        printf '%s\\n' $__ask_task_selector_cache\n")
	b.WriteString("        return 0\n")
	b.WriteString("    end\n")
	b.WriteString("    set -l selectors (command $ask_bin complete-uuids 2>/dev/null)\n")
	b.WriteString("    if test $status -ne 0\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    set -g __ask_task_selector_cache $selectors\n")
	b.WriteString("    set -g __ask_task_selector_cache_until (math $now + 2)\n")
	b.WriteString("    printf '%s\\n' $selectors\n")
	b.WriteString("end\n\n")
}

func writeFishAddDependencyModifierFunction(b *strings.Builder) {
	b.WriteString("function __ask_add_dependency_modifiers\n")
	b.WriteString("    set -l current (commandline -ct)\n")
	b.WriteString("    if test $current = depends\n")
	b.WriteString("        printf '%s\\n' 'depends:'\n")
	b.WriteString("        return 0\n")
	b.WriteString("    end\n")
	b.WriteString("    if not string match -qr '^depends:' -- $current\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    set -l raw (string sub -s 9 -- $current)\n")
	b.WriteString("    set -l partial $raw\n")
	b.WriteString("    set -l chosen\n")
	b.WriteString("    if string match -q '*,*' -- $raw\n")
	b.WriteString("        set -l pieces (string split ',' -- $raw)\n")
	b.WriteString("        set partial $pieces[-1]\n")
	b.WriteString("        if test (count $pieces) -gt 1\n")
	b.WriteString("            set chosen $pieces[1..-2]\n")
	b.WriteString("        end\n")
	b.WriteString("    end\n")
	b.WriteString("    for selector in (__ask_task_selectors)\n")
	b.WriteString("        if contains -- $selector $chosen\n")
	b.WriteString("            continue\n")
	b.WriteString("        end\n")
	b.WriteString("        if not string match -q -- \"$partial*\" $selector\n")
	b.WriteString("            continue\n")
	b.WriteString("        end\n")
	b.WriteString("        if test (count $chosen) -eq 0\n")
	b.WriteString("            printf 'depends:%s\\n' $selector\n")
	b.WriteString("        else\n")
	b.WriteString("            printf 'depends:%s,%s\\n' (string join ',' $chosen) $selector\n")
	b.WriteString("        end\n")
	b.WriteString("    end\n")
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
	b.WriteString("' -a '(__ask_task_selectors)' -d '")
	b.WriteString(strings.ReplaceAll(description, "'", "\\'"))
	b.WriteString("'\n")
}

func writeFishFunctionCompletionLine(b *strings.Builder, condition, functionName, description string) {
	b.WriteString("complete -c ask -n '")
	b.WriteString(condition)
	b.WriteString("' -a '(")
	b.WriteString(functionName)
	b.WriteString(")' -d '")
	b.WriteString(strings.ReplaceAll(description, "'", "\\'"))
	b.WriteString("'\n")
}

func quoteFishString(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"$", "\\$",
	)
	return `"` + replacer.Replace(value) + `"`
}
