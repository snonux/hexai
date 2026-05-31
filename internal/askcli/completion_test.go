package askcli

import (
	"strings"
	"testing"
)

func TestFishCompletion_IncludesCommandsAndExcludesExport(t *testing.T) {
	script := FishCompletion()
	for _, name := range []string{"na", "no-agent", "proj:"} {
		if !strings.Contains(script, " -a '"+name+"' ") {
			t.Fatalf("script missing scope completion for %q", name)
		}
	}
	for _, name := range []string{"add", "list", "all", "ready", "completed", "info", "annotate", "start", "stop", "done", "priority", "tag", "dep", "urgency", "modify", "denotate", "delete", "fish", "help"} {
		if !strings.Contains(script, " -a '"+name+"' ") {
			t.Fatalf("script missing root completion for %q", name)
		}
	}
	for _, line := range []string{
		"# Source with: ask fish | source",
		"complete -c ask -n '__ask_in_dep_context' -a 'add' -d 'Add a dependency'",
		"complete -c ask -n '__ask_in_dep_context' -a 'rm' -d 'Remove a dependency'",
		"complete -c ask -n '__ask_in_dep_context' -a 'list' -d 'List dependencies'",
		"function __ask_command_positionals",
		"function __ask_scope_prefix",
		"function __ask_task_selectors",
		"function __ask_add_dependency_modifiers",
		`set -l ask_bin "ask"`,
		"set -l selectors",
		"set selectors (command $ask_bin complete-aliases 2>/dev/null)",
		"set selectors (command $ask_bin $scope_prefix complete-aliases 2>/dev/null)",
		"case na no-agent 'proj:*'",
		"set cache_key (string join ' ' $scope_prefix)",
		"complete -c ask -n '__ask_in_uuid_context' -a '(__ask_task_selectors)' -d 'Task selector'",
		"complete -c ask -n '__ask_in_dep_uuid_context' -a '(__ask_task_selectors)' -d 'Task selector'",
		"complete -c ask -n '__ask_in_add_dep_modifier_context' -a '(__ask_add_dependency_modifiers)' -d 'Task dependency'",
		// The dep modifier function must extract just the selector (before the
		// tab) from each tab-separated "selector\tdescription" completion item.
		"for item in (__ask_task_selectors)",
		"set -l selector (string split -m1 \"\t\" -- $item)[1]",
	} {
		if !strings.Contains(script, line) {
			t.Fatalf("script missing dep completion line %q", line)
		}
	}
	if strings.Contains(script, "set -l selector (string split -m1 '\\t' -- $item)[1]") {
		t.Fatalf("dep selector split must use a real tab delimiter, not literal \\\\t")
	}
	if strings.Contains(script, "ask export") {
		t.Fatalf("script should not advertise non-existent export command")
	}
	if strings.Contains(script, "assets/ask.fish") {
		t.Fatalf("script should not reference a static asset")
	}
	for _, name := range []string{"info", "annotate", "start", "stop", "done", "priority", "tag", "modify", "denotate", "delete"} {
		if strings.Contains(script, "complete -c ask -n '__ask_in_uuid_context' -a '"+name+"'") {
			t.Fatalf("script should not hard-code UUID completion item %q", name)
		}
	}
}

func TestFishSingleSelectorCompletionContext(t *testing.T) {
	testCases := []struct {
		name       string
		positional []string
		want       bool
	}{
		{name: "info expects selector", positional: []string{"info"}, want: true},
		{name: "info expects selector with no-agent prefix", positional: []string{"na", "info"}, want: true},
		{name: "info expects selector with project prefix", positional: []string{"proj:alpha", "info"}, want: true},
		{name: "annotate expects selector", positional: []string{"annotate"}, want: true},
		{name: "priority expects selector", positional: []string{"priority"}, want: true},
		{name: "delete expects selector", positional: []string{"delete"}, want: true},
		{name: "delete expects selector with alias prefix", positional: []string{"no-agent", "delete"}, want: true},
		{name: "annotate stops after selector", positional: []string{"annotate", "0"}, want: false},
		{name: "priority stops after selector", positional: []string{"priority", "0"}, want: false},
		{name: "modify stops after selector", positional: []string{"modify", "0"}, want: false},
		{name: "dep is not a single selector command", positional: []string{"dep"}, want: false},
		{name: "empty positional", positional: nil, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fishSingleSelectorCompletionContext(tc.positional); got != tc.want {
				t.Fatalf("fishSingleSelectorCompletionContext(%v) = %t, want %t", tc.positional, got, tc.want)
			}
		})
	}
}

func TestFishDepSelectorCompletionContext(t *testing.T) {
	testCases := []struct {
		name       string
		positional []string
		want       bool
	}{
		{name: "dep add first selector", positional: []string{"dep", "add"}, want: true},
		{name: "dep add first selector with no-agent prefix", positional: []string{"na", "dep", "add"}, want: true},
		{name: "dep add first selector with project prefix", positional: []string{"proj:alpha", "dep", "add"}, want: true},
		{name: "dep add second selector", positional: []string{"dep", "add", "0"}, want: true},
		{name: "dep add stops after second selector", positional: []string{"dep", "add", "0", "1"}, want: false},
		{name: "dep rm first selector", positional: []string{"dep", "rm"}, want: true},
		{name: "dep rm second selector", positional: []string{"dep", "rm", "0"}, want: true},
		{name: "dep rm stops after second selector", positional: []string{"dep", "rm", "0", "1"}, want: false},
		{name: "dep list selector", positional: []string{"dep", "list"}, want: true},
		{name: "dep list stops after selector", positional: []string{"dep", "list", "0"}, want: false},
		{name: "dep unknown operation", positional: []string{"dep", "noop"}, want: false},
		{name: "non dep command", positional: []string{"info"}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fishDepSelectorCompletionContext(tc.positional); got != tc.want {
				t.Fatalf("fishDepSelectorCompletionContext(%v) = %t, want %t", tc.positional, got, tc.want)
			}
		})
	}
}

func TestFishAddDependencyModifierCompletionContext(t *testing.T) {
	testCases := []struct {
		name       string
		positional []string
		current    string
		want       bool
	}{
		{name: "add without depends modifier", positional: []string{"add", "task"}, current: "task", want: false},
		{name: "add with depends keyword prefix", positional: []string{"add"}, current: "depends", want: true},
		{name: "add with depends modifier", positional: []string{"add", "+cli"}, current: "depends:0", want: true},
		{name: "add with depends modifier and no-agent prefix", positional: []string{"na", "add", "+cli"}, current: "depends:0", want: true},
		{name: "add with depends modifier and project prefix", positional: []string{"proj:alpha", "add", "+cli"}, current: "depends:0", want: true},
		{name: "add with comma continuation", positional: []string{"add", "+cli"}, current: "depends:0,", want: true},
		{name: "non add command", positional: []string{"dep", "add"}, current: "depends:0", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fishAddDependencyModifierCompletionContext(tc.positional, tc.current); got != tc.want {
				t.Fatalf("fishAddDependencyModifierCompletionContext(%v, %q) = %t, want %t", tc.positional, tc.current, got, tc.want)
			}
		})
	}
}

func TestFishCompletionFor_EmbedsBinaryPath(t *testing.T) {
	script := FishCompletionFor(`/tmp/ask "$HOME"`)
	for _, line := range []string{
		`set -l ask_bin "/tmp/ask \"\$HOME\""`,
		"set selectors (command $ask_bin complete-aliases 2>/dev/null)",
	} {
		if !strings.Contains(script, line) {
			t.Fatalf("script missing %q", line)
		}
	}
}
