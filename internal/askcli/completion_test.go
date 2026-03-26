package askcli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFishCompletion_MatchesAsset(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	assetPath := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "assets", "ask.fish"))
	want, err := os.ReadFile(assetPath)
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	got := FishCompletion()
	if got != string(want) {
		t.Fatalf("fish completion asset mismatch\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}

func TestFishCompletion_IncludesCommandsAndExcludesExport(t *testing.T) {
	script := FishCompletion()
	for _, name := range []string{"add", "list", "all", "ready", "info", "annotate", "start", "stop", "done", "priority", "tag", "dep", "urgency", "modify", "denotate", "delete", "help"} {
		if !strings.Contains(script, " -a '"+name+"' ") {
			t.Fatalf("script missing root completion for %q", name)
		}
	}
	for _, line := range []string{
		"complete -c ask -n '__ask_in_dep_context' -a 'add' -d 'Add a dependency'",
		"complete -c ask -n '__ask_in_dep_context' -a 'rm' -d 'Remove a dependency'",
		"complete -c ask -n '__ask_in_dep_context' -a 'list' -d 'List dependencies'",
		"function __ask_task_uuids",
		"command ask all --json 2>/dev/null | jq -r '.[] | select(.status != \"completed\" and .status != \"deleted\") | .uuid' 2>/dev/null",
		"complete -c ask -n '__ask_in_uuid_context' -a '(__ask_task_uuids)' -d 'Show task details'",
		"complete -c ask -n '__ask_in_dep_uuid_context' -a '(__ask_task_uuids)' -d 'Add a dependency'",
		"complete -c ask -n '__ask_in_dep_uuid_context' -a '(__ask_task_uuids)' -d 'Remove a dependency'",
		"complete -c ask -n '__ask_in_dep_uuid_context' -a '(__ask_task_uuids)' -d 'List dependencies'",
	} {
		if !strings.Contains(script, line) {
			t.Fatalf("script missing dep completion line %q", line)
		}
	}
	if strings.Contains(script, "ask export") {
		t.Fatalf("script should not advertise non-existent export command")
	}
}
