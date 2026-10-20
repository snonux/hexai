package askcli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeCompletionFixture(t *testing.T, dir, name, selector string) string {
	t.Helper()
	binary := filepath.Join(dir, name)
	script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '" + selector + "\\t" + name + " task\\n'\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	completion := filepath.Join(dir, name+".fish")
	if err := os.WriteFile(completion, []byte(FishCompletionFor(binary)), 0o600); err != nil {
		t.Fatal(err)
	}
	return completion
}

func TestFishCompletionNamespacesSeparateTaskScopes(t *testing.T) {
	for _, tool := range []string{"fish", "bash"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}
	for _, order := range [][]string{{"ask", "nask"}, {"nask", "ask"}} {
		t.Run(strings.Join(order, "-then-"), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "__ask_tools")
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			files := map[string]string{
				"ask":  writeCompletionFixture(t, dir, "ask", "agent1"),
				"nask": writeCompletionFixture(t, dir, "nask", "personal1"),
			}
			var script strings.Builder
			for _, name := range order {
				fmt.Fprintf(&script, "source %s\n", quoteFishString(files[name]))
			}
			// Query both commands twice without delay to exercise cached results.
			for _, name := range []string{"ask", "nask", "ask", "nask"} {
				fmt.Fprintf(&script, "complete -C '%s info '\n", name)
			}
			cmd := exec.Command("fish", "--no-config", "-c", script.String())
			var stderr strings.Builder
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil || stderr.Len() > 0 {
				t.Fatalf("fish = %v, stderr = %s", err, stderr.String())
			}
			want := "agent1\task task\npersonal1\tnask task\nagent1\task task\npersonal1\tnask task\n"
			if string(out) != want {
				t.Fatalf("completion = %q, want %q", out, want)
			}
		})
	}
}
