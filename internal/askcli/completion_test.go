package askcli

import (
	"strings"
	"testing"
)

func TestFishCompletion_IncludesCommandsAndExcludesExport(t *testing.T) {
	script := FishCompletion()
	for _, name := range []string{"add", "list", "all", "ready", "info", "annotate", "start", "stop", "done", "priority", "tag", "dep", "urgency", "modify", "denotate", "delete", "fish", "help"} {
		if !strings.Contains(script, " -a '"+name+"' ") {
			t.Fatalf("script missing root completion for %q", name)
		}
	}
	for _, line := range []string{
		"# Source with: ask fish | source",
		"complete -c ask -n '__ask_in_dep_context' -a 'add' -d 'Add a dependency'",
		"complete -c ask -n '__ask_in_dep_context' -a 'rm' -d 'Remove a dependency'",
		"complete -c ask -n '__ask_in_dep_context' -a 'list' -d 'List dependencies'",
		"function __ask_task_uuids",
		`set -l ask_bin "ask"`,
		"set -l uuids (command $ask_bin complete-uuids 2>/dev/null)",
		"complete -c ask -n '__ask_in_uuid_context' -a '(__ask_task_uuids)' -d 'Task UUID'",
		"complete -c ask -n '__ask_in_dep_uuid_context' -a '(__ask_task_uuids)' -d 'Task UUID'",
	} {
		if !strings.Contains(script, line) {
			t.Fatalf("script missing dep completion line %q", line)
		}
	}
	if strings.Contains(script, "ask export") {
		t.Fatalf("script should not advertise non-existent export command")
	}
	if strings.Contains(script, "assets/ask.fish") {
		t.Fatalf("script should not reference a static asset")
	}
}

func TestFishCompletionFor_EmbedsBinaryPath(t *testing.T) {
	script := FishCompletionFor(`/tmp/ask "$HOME"`)
	for _, line := range []string{
		`set -l ask_bin "/tmp/ask \"\$HOME\""`,
		"set -l uuids (command $ask_bin complete-uuids 2>/dev/null)",
	} {
		if !strings.Contains(script, line) {
			t.Fatalf("script missing %q", line)
		}
	}
}
