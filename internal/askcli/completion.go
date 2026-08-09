package askcli

import (
	"path/filepath"
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
	positional = trimTaskPrefixes(positional)
	return evalCompletionContext(uuidContextSpec, positional, "", commandRegistry.singleSelectorNames())
}

func fishDepSelectorCompletionContext(positional []string) bool {
	positional = trimTaskPrefixes(positional)
	return evalCompletionContext(depUUIDContextSpec, positional, "", nil)
}

func fishAddDependencyModifierCompletionContext(positional []string, current string) bool {
	positional = trimTaskPrefixes(positional)
	return evalCompletionContext(addDepModifierContextSpec, positional, strings.TrimSpace(current), nil)
}

// FishCompletion returns the default Fish completion script for the ask CLI.
func FishCompletion() string {
	return FishCompletionFor("ask")
}

// fishShellCommandName returns the argv0 basename used for `complete -c` wiring.
func fishShellCommandName(binaryPath string) string {
	first := strings.TrimSpace(binaryPath)
	if i := strings.IndexByte(first, ' '); i >= 0 {
		first = first[:i]
	}
	return filepath.Base(first)
}

// FishCompletionFor returns a Fish completion script that points to the provided binary path.
func FishCompletionFor(binaryPath string) string {
	cmd := fishShellCommandName(binaryPath)
	var b strings.Builder
	writeFishPreamble(&b, cmd)
	writeFishContextFunctions(&b)
	writeFishTaskSelectorFunction(&b, binaryPath)
	writeFishAddDependencyModifierFunction(&b)
	b.WriteString("complete -c ")
	b.WriteString(cmd)
	b.WriteString(" -f\n")
	b.WriteString("complete -c ")
	b.WriteString(cmd)
	b.WriteString(" -s j -l json -d 'Emit JSON output'\n")
	for _, item := range []fishCompletionItem{
		{name: "na", description: "Run against project tasks without +agent"},
		{name: "no-agent", description: "Run against project tasks without +agent"},
		{name: "proj:", description: "Run against an explicit project"},
	} {
		writeFishCompletionLine(&b, cmd, "__ask_needs_root_completion", item)
	}
	for _, entry := range commandRegistry.rootCompletionEntries() {
		item := fishCompletionItem{name: entry.name, description: entry.description}
		writeFishCompletionLine(&b, cmd, "__ask_needs_command_completion", item)
	}
	for _, item := range askDepCompletionItems {
		writeFishCompletionLine(&b, cmd, "__ask_in_dep_context", item)
	}
	writeFishUUIDCompletionLine(&b, cmd, "__ask_in_uuid_context", "Task selector")
	writeFishUUIDCompletionLine(&b, cmd, "__ask_in_dep_uuid_context", "Task selector")
	writeFishFunctionCompletionLine(&b, cmd, "__ask_in_add_dep_modifier_context", "__ask_add_dependency_modifiers", "Task dependency")
	return b.String()
}

func writeFishPreamble(b *strings.Builder, cmd string) {
	b.WriteString("# Fish completion for ")
	b.WriteString(cmd)
	b.WriteString(".\n# Source with: ")
	b.WriteString(cmd)
	b.WriteString(" fish | source\n\n")
}

func writeFishContextFunctions(b *strings.Builder) {
	writeFishPositionalTokensFunction(b)
	writeFishCommandPositionalsFunction(b)
	writeFishScopePrefixFunction(b)
	writeFishNeedsRootCompletionFunction(b)
	writeFishNeedsCommandCompletionFunction(b)
	writeFishDepContextFunction(b)
	names := commandRegistry.singleSelectorNames()
	for _, spec := range completionContextSpecs {
		writeFishContextFunctionFromSpec(b, spec, names)
	}
}

func writeFishPositionalTokensFunction(b *strings.Builder) {
	b.WriteString("function __ask_positional_tokens\n")
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
	b.WriteString("function __ask_command_positionals\n")
	b.WriteString("    set -l positional (__ask_positional_tokens)\n")
	b.WriteString("    while test (count $positional) -gt 0\n")
	b.WriteString("        switch $positional[1]\n")
	b.WriteString("            case na no-agent 'proj:*'\n")
	b.WriteString("                set positional $positional[2..-1]\n")
	b.WriteString("            case '*'\n")
	b.WriteString("                break\n")
	b.WriteString("        end\n")
	b.WriteString("    end\n")
	b.WriteString("    for token in $positional\n")
	b.WriteString("        printf '%s\\n' $token\n")
	b.WriteString("    end\n")
	b.WriteString("end\n\n")
}

func writeFishScopePrefixFunction(b *strings.Builder) {
	b.WriteString("function __ask_scope_prefix\n")
	b.WriteString("    set -l positional (__ask_positional_tokens)\n")
	b.WriteString("    while test (count $positional) -gt 0\n")
	b.WriteString("        switch $positional[1]\n")
	b.WriteString("            case na no-agent 'proj:*'\n")
	b.WriteString("                printf '%s\\n' $positional[1]\n")
	b.WriteString("                set positional $positional[2..-1]\n")
	b.WriteString("            case '*'\n")
	b.WriteString("                return 0\n")
	b.WriteString("        end\n")
	b.WriteString("    end\n")
	b.WriteString("    return 0\n")
	b.WriteString("end\n\n")
}

func writeFishNeedsRootCompletionFunction(b *strings.Builder) {
	b.WriteString("function __ask_needs_root_completion\n")
	b.WriteString("    set -l positional (__ask_positional_tokens)\n")
	b.WriteString("    if test (count $positional) -eq 0\n")
	b.WriteString("        return 0\n")
	b.WriteString("    end\n")
	b.WriteString("    return 1\n")
	b.WriteString("end\n\n")
}

func writeFishNeedsCommandCompletionFunction(b *strings.Builder) {
	b.WriteString("function __ask_needs_command_completion\n")
	b.WriteString("    set -l positional (__ask_positional_tokens)\n")
	b.WriteString("    if test (count $positional) -eq 0\n")
	b.WriteString("        return 0\n")
	b.WriteString("    end\n")
	b.WriteString("    if test (count $positional) -eq 1\n")
	b.WriteString("        switch $positional[1]\n")
	b.WriteString("            case na no-agent 'proj:*'\n")
	b.WriteString("                return 0\n")
	b.WriteString("        end\n")
	b.WriteString("    end\n")
	b.WriteString("    return 1\n")
	b.WriteString("end\n\n")
}

func writeFishDepContextFunction(b *strings.Builder) {
	b.WriteString("function __ask_in_dep_context\n")
	b.WriteString("    set -l positional (__ask_command_positionals)\n")
	b.WriteString("    if test (count $positional) -lt 1\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    if test $positional[1] != dep\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    test (count $positional) -eq 1\n")
	b.WriteString("end\n\n")
}

func writeFishTaskSelectorFunction(b *strings.Builder, binaryPath string) {
	b.WriteString("function __ask_task_selectors\n")
	b.WriteString("    set -l ask_bin ")
	b.WriteString(quoteFishString(binaryPath))
	b.WriteString("\n")
	b.WriteString("    set -l scope_prefix (__ask_scope_prefix)\n")
	b.WriteString("    set -l cache_key default\n")
	b.WriteString("    if test (count $scope_prefix) -gt 0\n")
	b.WriteString("        set cache_key (string join ' ' $scope_prefix)\n")
	b.WriteString("    end\n")
	b.WriteString("    set -l now (date +%s)\n")
	b.WriteString("    if set -q __ask_task_selector_cache_until; and test $__ask_task_selector_cache_until -ge $now; and set -q __ask_task_selector_cache_key; and test \"$__ask_task_selector_cache_key\" = \"$cache_key\"\n")
	b.WriteString("        printf '%s\\n' $__ask_task_selector_cache\n")
	b.WriteString("        return 0\n")
	b.WriteString("    end\n")
	b.WriteString("    set -l selectors\n")
	b.WriteString("    if test (count $scope_prefix) -gt 0\n")
	b.WriteString("        set selectors (command $ask_bin $scope_prefix complete-aliases 2>/dev/null)\n")
	b.WriteString("    else\n")
	b.WriteString("        set selectors (command $ask_bin complete-aliases 2>/dev/null)\n")
	b.WriteString("    end\n")
	b.WriteString("    if test $status -ne 0\n")
	b.WriteString("        return 1\n")
	b.WriteString("    end\n")
	b.WriteString("    set -g __ask_task_selector_cache $selectors\n")
	b.WriteString("    set -g __ask_task_selector_cache_until (math $now + 2)\n")
	b.WriteString("    set -g __ask_task_selector_cache_key $cache_key\n")
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
	// Each item from __ask_task_selectors is "selector\tdescription"; extract
	// just the selector (before the tab) for matching and output purposes.
	b.WriteString("    for item in (__ask_task_selectors)\n")
	b.WriteString("        set -l selector (string split -m1 \"\t\" -- $item)[1]\n")
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

func writeFishCompletionLine(b *strings.Builder, cmd, condition string, item fishCompletionItem) {
	b.WriteString("complete -c ")
	b.WriteString(cmd)
	b.WriteString(" -n '")
	b.WriteString(condition)
	b.WriteString("' -a '")
	b.WriteString(item.name)
	b.WriteString("' -d '")
	b.WriteString(strings.ReplaceAll(item.description, "'", "\\'"))
	b.WriteString("'\n")
}

func writeFishUUIDCompletionLine(b *strings.Builder, cmd, condition, description string) {
	b.WriteString("complete -c ")
	b.WriteString(cmd)
	b.WriteString(" -n '")
	b.WriteString(condition)
	b.WriteString("' -a '(__ask_task_selectors)' -d '")
	b.WriteString(strings.ReplaceAll(description, "'", "\\'"))
	b.WriteString("'\n")
}

func writeFishFunctionCompletionLine(b *strings.Builder, cmd, condition, functionName, description string) {
	b.WriteString("complete -c ")
	b.WriteString(cmd)
	b.WriteString(" -n '")
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
