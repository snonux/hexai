package askcli

import (
	"strings"
)

type fishCompletionItem struct {
	name        string
	description string
}

var askDepCompletionItems = []fishCompletionItem{
	{name: "add", description: "Add a dependency"},
	{name: "rm", description: "Remove a dependency"},
	{name: "list", description: "List dependencies"},
}

func fishSingleSelectorCompletionContext(positional []string) bool {
	positional = trimTaskScopePrefix(positional)
	if len(positional) != 1 {
		return false
	}

	for _, command := range commandRegistry.singleSelectorNames() {
		if positional[0] == command {
			return true
		}
	}
	return false
}

func fishDepSelectorCompletionContext(positional []string) bool {
	positional = trimTaskScopePrefix(positional)
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
	positional = trimTaskScopePrefix(positional)
	if len(positional) == 0 || positional[0] != "add" {
		return false
	}
	current = strings.TrimSpace(current)
	return current == "depends" || strings.HasPrefix(current, "depends:")
}

// FishCompletion returns the default Fish completion script for the do CLI.
func FishCompletion() string {
	return FishCompletionFor("do")
}

// FishCompletionFor returns a Fish completion script that points to the provided binary path.
func FishCompletionFor(binaryPath string) string {
	var b strings.Builder
	writeFishPreamble(&b)
	writeFishContextFunctions(&b)
	writeFishTaskSelectorFunction(&b, binaryPath)
	writeFishAddDependencyModifierFunction(&b)
	b.WriteString("complete -c do -f\n")
	b.WriteString("complete -c do -s j -l json -d 'Emit JSON output'\n")
	for _, item := range []fishCompletionItem{
		{name: "na", description: "Run against project tasks without +agent"},
		{name: "no-agent", description: "Run against project tasks without +agent"},
	} {
		writeFishCompletionLine(&b, "__do_needs_root_completion", item)
	}
	for _, entry := range commandRegistry.rootCompletionEntries() {
		item := fishCompletionItem{name: entry.name, description: entry.description}
		writeFishCompletionLine(&b, "__do_needs_command_completion", item)
	}
	for _, item := range askDepCompletionItems {
		writeFishCompletionLine(&b, "__do_in_dep_context", item)
	}
	writeFishUUIDCompletionLine(&b, "__do_in_uuid_context", "Task selector")
	writeFishUUIDCompletionLine(&b, "__do_in_dep_uuid_context", "Task selector")
	writeFishFunctionCompletionLine(&b, "__do_in_add_dep_modifier_context", "__do_add_dependency_modifiers", "Task dependency")
	return b.String()
}

func writeFishPreamble(b *strings.Builder) {
	b.WriteString("# Fish completion for do.\n")
	b.WriteString("# Source with: do fish | source\n\n")
}

func writeFishContextFunctions(b *strings.Builder) {
	writeFishPositionalTokensFunction(b)
	writeFishCommandPositionalsFunction(b)
	writeFishScopePrefixFunction(b)
	writeFishNeedsRootCompletionFunction(b)
	writeFishNeedsCommandCompletionFunction(b)
	writeFishDepContextFunction(b)
	writeFishUUIDContextFunction(b)
	writeFishDepUUIDContextFunction(b)
	writeFishAddDependencyModifierContextFunction(b)
}

func writeFishPositionalTokensFunction(b *strings.Builder) {
	b.WriteString("function __do_positional_tokens\n")
	b.WriteString("    set -l tokens (commandline -opc)\n")
	b.WriteString("    set -l positional\n")
	b.WriteString("    for token in $tokens[2..-1]\n")
	b.WriteString("        if string match -qr '^-' -- $token\n")
	b.WriteString("            continue\n")
	b.WriteString("        end\n")
	b.WriteString("        set -a positional $token\n")
	b.WriteString("    end\n")
	b.WriteString("    for token in $positional\n")
	b.WriteString("        printf '%s\\n' $token\n")
	b.WriteString("    end\n")
	b.WriteString("end\n\n")
}

func writeFishCommandPositionalsFunction(b *strings.Builder) {
	b.WriteString("function __do_command_positionals\n")
	b.WriteString("    set -l positional (__do_positional_tokens)\n")
	b.WriteString("    if test (count $positional) -gt 0\n")
	b.WriteString("        switch $positional[1]\n")
	b.WriteString("            case na no-agent\n")
	b.WriteString("                for token in $positional[2..-1]\n")
	b.WriteString("                    printf '%s\\n' $token\n")
	b.WriteString("                end\n")
	b.WriteString("                return 0\n")
	b.WriteString("        end\n")
	b.WriteString("    end\n")
	b.WriteString("    for token in $positional\n")
	b.WriteString("        printf '%s\\n' $token\n")
	b.WriteString("    end\n")
	b.WriteString("end\n\n")
}

func writeFishScopePrefixFunction(b *strings.Builder) {
	b.WriteString("function __do_scope_prefix\n")
	b.WriteString("    set -l positional (__do_positional_tokens)\n")
	b.WriteString("    if test (count $positional) -eq 0\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    switch $positional[1]\n")
	b.WriteString("        case na no-agent\n")
	b.WriteString("            printf '%s\\n' $positional[1]\n")
	b.WriteString("            return 0\n")
	b.WriteString("        case '*'\n")
	b.WriteString("            return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    return 1\n")
	b.WriteString("end\n\n")
}

func writeFishNeedsRootCompletionFunction(b *strings.Builder) {
	b.WriteString("function __do_needs_root_completion\n")
	b.WriteString("    set -l positional (__do_positional_tokens)\n")
	b.WriteString("    if test (count $positional) -eq 0\n")
	b.WriteString("        return 0\n")
	b.WriteString("    end\n")
	b.WriteString("    return 1\n")
	b.WriteString("end\n\n")
}

func writeFishNeedsCommandCompletionFunction(b *strings.Builder) {
	b.WriteString("function __do_needs_command_completion\n")
	b.WriteString("    set -l positional (__do_positional_tokens)\n")
	b.WriteString("    if test (count $positional) -eq 0\n")
	b.WriteString("        return 0\n")
	b.WriteString("    end\n")
	b.WriteString("    if test (count $positional) -eq 1\n")
	b.WriteString("        switch $positional[1]\n")
	b.WriteString("            case na no-agent\n")
	b.WriteString("                return 0\n")
	b.WriteString("        end\n")
	b.WriteString("    end\n")
	b.WriteString("    return 1\n")
	b.WriteString("end\n\n")
}

func writeFishDepContextFunction(b *strings.Builder) {
	b.WriteString("function __do_in_dep_context\n")
	b.WriteString("    set -l positional (__do_command_positionals)\n")
	b.WriteString("    if test (count $positional) -lt 1\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    if test $positional[1] != dep\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    test (count $positional) -eq 1\n")
	b.WriteString("end\n\n")
}

func writeFishUUIDContextFunction(b *strings.Builder) {
	b.WriteString("function __do_in_uuid_context\n")
	b.WriteString("    set -l positional (__do_command_positionals)\n")
	b.WriteString("    if test (count $positional) -eq 0\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    if test (count $positional) -ne 1\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    switch $positional[1]\n")
	b.WriteString("        case ")
	b.WriteString(strings.Join(commandRegistry.singleSelectorNames(), " "))
	b.WriteString("\n")
	b.WriteString("            return 0\n")
	b.WriteString("        case '*'\n")
	b.WriteString("            return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    return 1\n")
	b.WriteString("end\n\n")
}

func writeFishDepUUIDContextFunction(b *strings.Builder) {
	b.WriteString("function __do_in_dep_uuid_context\n")
	b.WriteString("    set -l positional (__do_command_positionals)\n")
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
	b.WriteString("function __do_in_add_dep_modifier_context\n")
	b.WriteString("    set -l positional (__do_command_positionals)\n")
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
	b.WriteString("function __do_task_selectors\n")
	b.WriteString("    set -l do_bin ")
	b.WriteString(quoteFishString(binaryPath))
	b.WriteString("\n")
	b.WriteString("    set -l scope_prefix (__do_scope_prefix)\n")
	b.WriteString("    set -l cache_key default\n")
	b.WriteString("    if test -n \"$scope_prefix\"\n")
	b.WriteString("        set cache_key $scope_prefix\n")
	b.WriteString("    end\n")
	b.WriteString("    set -l now (date +%s)\n")
	b.WriteString("    if set -q __do_task_selector_cache_until; and test $__do_task_selector_cache_until -ge $now; and set -q __do_task_selector_cache_key; and test \"$__do_task_selector_cache_key\" = \"$cache_key\"\n")
	b.WriteString("        printf '%s\\n' $__do_task_selector_cache\n")
	b.WriteString("        return 0\n")
	b.WriteString("    end\n")
	b.WriteString("    set -l selectors\n")
	b.WriteString("    if test -n \"$scope_prefix\"\n")
	b.WriteString("        set selectors (command $do_bin $scope_prefix complete-aliases 2>/dev/null)\n")
	b.WriteString("    else\n")
	b.WriteString("        set selectors (command $do_bin complete-aliases 2>/dev/null)\n")
	b.WriteString("    end\n")
	b.WriteString("    if test $status -ne 0\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    set -g __do_task_selector_cache $selectors\n")
	b.WriteString("    set -g __do_task_selector_cache_until (math $now + 2)\n")
	b.WriteString("    set -g __do_task_selector_cache_key $cache_key\n")
	b.WriteString("    printf '%s\\n' $selectors\n")
	b.WriteString("end\n\n")
}

func writeFishAddDependencyModifierFunction(b *strings.Builder) {
	b.WriteString("function __do_add_dependency_modifiers\n")
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
	// Each item from __do_task_selectors is "selector\tdescription"; extract
	// just the selector (before the tab) for matching and output purposes.
	b.WriteString("    for item in (__do_task_selectors)\n")
	b.WriteString("        set -l selector (string split -m1 '\\t' -- $item)[1]\n")
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
	b.WriteString("complete -c do -n '")
	b.WriteString(condition)
	b.WriteString("' -a '")
	b.WriteString(item.name)
	b.WriteString("' -d '")
	b.WriteString(strings.ReplaceAll(item.description, "'", "\\'"))
	b.WriteString("'\n")
}

func writeFishUUIDCompletionLine(b *strings.Builder, condition, description string) {
	b.WriteString("complete -c do -n '")
	b.WriteString(condition)
	b.WriteString("' -a '(__do_task_selectors)' -d '")
	b.WriteString(strings.ReplaceAll(description, "'", "\\'"))
	b.WriteString("'\n")
}

func writeFishFunctionCompletionLine(b *strings.Builder, condition, functionName, description string) {
	b.WriteString("complete -c do -n '")
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
