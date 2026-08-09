package askcli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestFishContextFunctions_MatchGoHelpers is the "tests cover what runs" guard
// for the spec-driven completion contexts. The three Fish condition functions
// (__ask_in_uuid_context, __ask_in_dep_uuid_context,
// __ask_in_add_dep_modifier_context) are GENERATED from the same
// completionContextSpec that the Go fish*CompletionContext helpers evaluate.
// This test sources the actual generated completion script, drives each
// generated function over a vector matrix, and asserts the Fish exit status
// matches the Go helper's boolean for every case — so the shell that runs in
// production is covered by the same vectors as the Go logic, and any future
// divergence between the spec's Go evaluation and its Fish emission is caught.
//
// It is skipped when fish is not installed (e.g. CI without fish). Positionals
// are pre-stripped with trimTaskPrefixes (the Go equivalent of the real
// __ask_command_positionals scope stripping) so the test exercises the shared
// context logic rather than the separate scope-prefix infrastructure.
func TestFishContextFunctions_MatchGoHelpers(t *testing.T) {
	if _, err := exec.LookPath("fish"); err != nil {
		t.Skip("fish not installed; skipping generated-shell execution test")
	}

	cases := buildFishContextParityCases()

	var b strings.Builder
	b.WriteString(FishCompletion())
	b.WriteString("\n# --- test harness overrides (do not ship) ---\n")
	b.WriteString("function __ask_command_positionals\n    for t in $__test_pos\n        echo $t\n    end\nend\n")
	b.WriteString("function commandline\n    echo -n -- $__test_current\nend\n")
	for _, c := range cases {
		writeFishParityCase(&b, c)
	}
	b.WriteString("echo ALL-OK\n")

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "ctx.fish")
	if err := os.WriteFile(scriptPath, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("fish", "--no-config", scriptPath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("fish run failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	// A fish runtime error (e.g. an unquoted empty variable expansion) can set
	// the function's exit status to 1 and coincidentally match the expected want,
	// masking a real divergence. Require stderr to be empty so such errors fail
	// the test rather than slip through as a noisy "false".
	if stderr.Len() > 0 {
		t.Fatalf("fish emitted stderr (a runtime error would mask divergence):\n%s", stderr.String())
	}
	outStr := stdout.String()
	if strings.Contains(outStr, "FAIL") || !strings.Contains(outStr, "ALL-OK") {
		t.Fatalf("fish context functions diverged from Go helpers:\n%s", outStr)
	}
}

type fishContextParityCase struct {
	fn         string
	positional []string
	current    string
	expected   bool
}

// writeFishParityCase appends a Fish block that drives the named context
// function with the case's positionals/current and emits a FAIL line when the
// Fish status differs from the Go helper's expected boolean.
func writeFishParityCase(b *strings.Builder, c fishContextParityCase) {
	trimmed := trimTaskPrefixes(c.positional)
	want := 1
	if c.expected {
		want = 0
	}
	fmt.Fprintf(b, "set -g __test_pos %s\n", strings.Join(trimmed, " "))
	fmt.Fprintf(b, "set -g __test_current %s\n", fishSingleQuote(c.current))
	fmt.Fprintf(b, "%s\n", c.fn)
	fmt.Fprintf(b, "set -l __got $status\n")
	fmt.Fprintf(b, "if test $__got -ne %d\n    echo \"FAIL %s pos=[%s] cur=%s got=$__got want=%d\"\nend\n",
		want, c.fn, strings.Join(c.positional, " "), fishSingleQuote(c.current), want)
}

// fishContextParityVectors is the vector matrix shared by the Go helpers and
// the generated Fish functions, expressed as raw inputs (expected is derived
// from the Go helper in buildFishContextParityCases so the test asserts Fish
// parity rather than hand-maintained expectations). It is a package-level var so
// the long literal does not count toward the function-length limit.
var fishContextParityVectors = []struct {
	fn         string
	positional []string
	current    string
}{
	// single-selector (uuid) context
	{"__ask_in_uuid_context", []string{"info"}, ""},
	{"__ask_in_uuid_context", []string{"na", "info"}, ""},
	{"__ask_in_uuid_context", []string{"proj:alpha", "info"}, ""},
	{"__ask_in_uuid_context", []string{"annotate"}, ""},
	{"__ask_in_uuid_context", []string{"priority"}, ""},
	{"__ask_in_uuid_context", []string{"delete"}, ""},
	{"__ask_in_uuid_context", []string{"no-agent", "delete"}, ""},
	{"__ask_in_uuid_context", []string{"annotate", "0"}, ""},
	{"__ask_in_uuid_context", []string{"priority", "0"}, ""},
	{"__ask_in_uuid_context", []string{"modify", "0"}, ""},
	{"__ask_in_uuid_context", []string{"dep"}, ""},
	{"__ask_in_uuid_context", nil, ""},
	// dep-uuid context
	{"__ask_in_dep_uuid_context", []string{"dep", "add"}, ""},
	{"__ask_in_dep_uuid_context", []string{"na", "dep", "add"}, ""},
	{"__ask_in_dep_uuid_context", []string{"proj:alpha", "dep", "add"}, ""},
	{"__ask_in_dep_uuid_context", []string{"dep", "add", "0"}, ""},
	{"__ask_in_dep_uuid_context", []string{"dep", "add", "0", "1"}, ""},
	{"__ask_in_dep_uuid_context", []string{"dep", "rm"}, ""},
	{"__ask_in_dep_uuid_context", []string{"dep", "rm", "0"}, ""},
	{"__ask_in_dep_uuid_context", []string{"dep", "rm", "0", "1"}, ""},
	{"__ask_in_dep_uuid_context", []string{"dep", "list"}, ""},
	{"__ask_in_dep_uuid_context", []string{"dep", "list", "0"}, ""},
	{"__ask_in_dep_uuid_context", []string{"dep", "noop"}, ""},
	{"__ask_in_dep_uuid_context", []string{"info"}, ""},
	{"__ask_in_dep_uuid_context", []string{"dep"}, ""},
	{"__ask_in_dep_uuid_context", nil, ""},
	// add-dep-modifier context
	{"__ask_in_add_dep_modifier_context", []string{"add", "task"}, "task"},
	{"__ask_in_add_dep_modifier_context", []string{"add"}, "depends"},
	{"__ask_in_add_dep_modifier_context", []string{"add", "+cli"}, "depends:0"},
	{"__ask_in_add_dep_modifier_context", []string{"na", "add", "+cli"}, "depends:0"},
	{"__ask_in_add_dep_modifier_context", []string{"proj:alpha", "add", "+cli"}, "depends:0"},
	{"__ask_in_add_dep_modifier_context", []string{"add", "+cli"}, "depends:0,"},
	{"__ask_in_add_dep_modifier_context", []string{"dep", "add"}, "depends:0"},
	{"__ask_in_add_dep_modifier_context", nil, "depends"},
	{"__ask_in_add_dep_modifier_context", []string{"add"}, " depends "},
	{"__ask_in_add_dep_modifier_context", []string{"add"}, "dependsX"},
	{"__ask_in_add_dep_modifier_context", []string{"add"}, ""},
}

// buildFishContextParityCases maps the raw vectors to expected booleans
// computed from the Go helpers, so the Fish execution test asserts parity
// rather than re-stating expected values by hand.
func buildFishContextParityCases() []fishContextParityCase {
	cases := make([]fishContextParityCase, 0, len(fishContextParityVectors))
	for _, r := range fishContextParityVectors {
		var expected bool
		switch r.fn {
		case "__ask_in_uuid_context":
			expected = fishSingleSelectorCompletionContext(r.positional)
		case "__ask_in_dep_uuid_context":
			expected = fishDepSelectorCompletionContext(r.positional)
		case "__ask_in_add_dep_modifier_context":
			expected = fishAddDependencyModifierCompletionContext(r.positional, r.current)
		}
		cases = append(cases, fishContextParityCase{
			fn: r.fn, positional: r.positional, current: r.current, expected: expected,
		})
	}
	return cases
}
