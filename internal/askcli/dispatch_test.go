package askcli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDispatcher_Help(t *testing.T) {
	d := NewDispatcher(nil)
	var stdout bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"help"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("help exit code = %d, want 0", code)
	}
	if err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "ask - task management CLI") {
		t.Fatalf("help missing title: %s", output)
	}
	if !strings.Contains(output, "ask proj:<name> <subcommand...>") {
		t.Fatalf("help missing project prefix: %s", output)
	}
	if !strings.Contains(output, "ask na <subcommand...>") || !strings.Contains(output, "ask no-agent <subcommand...>") {
		t.Fatalf("help missing no-agent scope prefixes: %s", output)
	}
	if !strings.Contains(output, "ask list") {
		t.Fatalf("help missing list subcommand: %s", output)
	}
	if !strings.Contains(output, "ask all") {
		t.Fatalf("help missing all subcommand: %s", output)
	}
	if !strings.Contains(output, "ask fish") {
		t.Fatalf("help missing fish subcommand: %s", output)
	}
}

func TestDispatcher_DefaultsInvalidSubcommandToAdd(t *testing.T) {
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC) }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	tests := []struct {
		name        string
		args        []string
		wantCall    []string
		wantProject string
	}{
		{
			name:     "plain",
			args:     []string{"foo", "bar", "baz"},
			wantCall: []string{"add", "rc.verbose=nothing", "rc.verbose=new-uuid", "foo bar baz"},
		},
		{
			name:        "project scoped",
			args:        []string{"proj:alpha", "foo", "bar", "baz"},
			wantCall:    []string{"add", "rc.verbose=nothing", "rc.verbose=new-uuid", "foo bar baz"},
			wantProject: "alpha",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotCall []string
			var gotProject string
			d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
				gotCall = append([]string(nil), args...)
				gotProject, _ = taskProjectFromContext(ctx)
				_, _ = io.WriteString(stdout, "Created task uuid-default-add.\n")
				return 0, nil
			}})

			var stdout, stderr bytes.Buffer
			code, err := d.Dispatch(context.Background(), tc.args, nil, &stdout, &stderr)
			if err != nil {
				t.Fatalf("Dispatch returned error: %v", err)
			}
			if code != 0 {
				t.Fatalf("Dispatch code = %d, want 0: stderr=%s", code, stderr.String())
			}
			if !reflect.DeepEqual(gotCall, tc.wantCall) {
				t.Fatalf("runner args = %v, want %v", gotCall, tc.wantCall)
			}
			if gotProject != tc.wantProject {
				t.Fatalf("project override = %q, want %q", gotProject, tc.wantProject)
			}
			if stdout.String() != "created task 0\n" {
				t.Fatalf("stdout = %q, want created task alias", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestDispatcher_RealSubcommandsDoNotDefaultToAdd(t *testing.T) {
	var gotCall []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		gotCall = append([]string(nil), args...)
		_, _ = io.WriteString(stdout, `[]`)
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"list", "+ready"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("Dispatch code = %d, want 0: stderr=%s", code, stderr.String())
	}
	if want := []string{"status:pending", "+ready", "export"}; !reflect.DeepEqual(gotCall, want) {
		t.Fatalf("runner args = %v, want %v", gotCall, want)
	}
	if !strings.Contains(stdout.String(), "Description") {
		t.Fatalf("stdout = %q, want rendered task list", stdout.String())
	}
}

func TestDispatcher_CompleteUUIDsSubcommand(t *testing.T) {
	// Use a temp dir for the alias cache so this test is hermetic and does
	// not depend on cache state left by other tests.
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC) }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if strings.Join(args, " ") != "status:pending export" {
			t.Fatalf("args = %v, want pending export", args)
		}
		_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1","description":"Sample task"}]`)
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"complete-uuids"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("complete-uuids returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("complete-uuids code = %d, want 0", code)
	}
	// Output is "selector\tdescription" for fish shell autocompletion display.
	if got := stdout.String(); got != "0\tSample task\nuuid-1\tSample task\n" {
		t.Fatalf("stdout = %q, want tab-separated selector list", got)
	}
}

func TestDispatcher_CompleteAliasesSubcommand(t *testing.T) {
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC) }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if strings.Join(args, " ") != "status:pending export" {
			t.Fatalf("args = %v, want pending export", args)
		}
		_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1","description":"Sample task"}]`)
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"complete-aliases"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("complete-aliases returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("complete-aliases code = %d, want 0", code)
	}
	// Fish completion uses aliases only; UUID lines are omitted.
	if got := stdout.String(); got != "0\tSample task\n" {
		t.Fatalf("stdout = %q, want alias-only tab-separated list", got)
	}
}

func TestDispatcher_LongHelp(t *testing.T) {
	d := NewDispatcher(nil)
	var stdout bytes.Buffer
	d.Dispatch(context.Background(), []string{"help"}, nil, &stdout, io.Discard)
	output := stdout.String()
	for _, sub := range []string{"add", "list", "all", "ready", "info", "annotate", "start", "stop", "done", "priority", "tag", "dep", "urgency", "watch", "modify", "denotate", "delete", "fish"} {
		if !strings.Contains(output, "ask "+sub) {
			t.Errorf("help missing subcommand: ask %s", sub)
		}
	}
}

func TestDispatcher_FishSubcommand(t *testing.T) {
	d := NewDispatcher(nil)
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"fish"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("fish returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("fish code = %d, want 0", code)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if got := stdout.String(); got != FishCompletionFor(exe) {
		t.Fatalf("fish output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, FishCompletionFor(exe))
	}
	if stderr.Len() != 0 {
		t.Fatalf("fish wrote unexpected stderr: %q", stderr.String())
	}
}

func TestDispatcher_FishSubcommandRejectsExtraArgs(t *testing.T) {
	d := NewDispatcher(nil)
	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"fish", "extra"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("fish extra args returned error: %v", err)
	}
	if code != 1 {
		t.Fatalf("fish extra args code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("fish extra args wrote unexpected stdout: %q", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, "usage: ask fish") {
		t.Fatalf("fish extra args stderr = %q, want usage", got)
	}
}

func TestDispatcher_DispatchPersistsJSONFlagOnDispatcher(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if strings.Join(args, " ") != "status:pending export" {
			t.Fatalf("args = %v, want list export args", args)
		}
		_, _ = io.WriteString(stdout, `[]`)
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"--json", "list"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("Dispatch code = %d, want 0", code)
	}
	if !d.jsonOutput {
		t.Fatal("Dispatch did not persist jsonOutput on the dispatcher")
	}
	if got := stdout.String(); got != "[]\n" {
		t.Fatalf("stdout = %q, want JSON output", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestParseGlobalFlags(t *testing.T) {
	filtered, jsonOutput := parseGlobalFlags([]string{"--json", "list", "--json", "extra"})
	if !jsonOutput {
		t.Fatalf("json flag not detected")
	}
	if got := strings.Join(filtered, " "); got != "list extra" {
		t.Fatalf("filtered args = %q, want \"list extra\"", got)
	}
}

func TestParseTaskScopePrefix(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantScope taskScopeMode
		wantArgs  []string
	}{
		{name: "default scope", args: []string{"list"}, wantScope: taskScopeAgent, wantArgs: []string{"list"}},
		{name: "na prefix", args: []string{"na", "list"}, wantScope: taskScopeNoAgent, wantArgs: []string{"list"}},
		{name: "no-agent prefix", args: []string{"no-agent", "info", "0"}, wantScope: taskScopeNoAgent, wantArgs: []string{"info", "0"}},
		{name: "empty args", args: nil, wantScope: taskScopeAgent, wantArgs: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotScope, gotArgs := parseTaskScopePrefix(tc.args)
			if gotScope != tc.wantScope {
				t.Fatalf("scope = %v, want %v", gotScope, tc.wantScope)
			}
			if !reflect.DeepEqual(gotArgs, tc.wantArgs) {
				t.Fatalf("args = %v, want %v", gotArgs, tc.wantArgs)
			}
		})
	}
}

func TestParseTaskPrefixes(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantScope      taskScopeMode
		wantProject    string
		wantProjectSet bool
		wantRemaining  []string
	}{
		{name: "default", args: []string{"list"}, wantScope: taskScopeAgent, wantRemaining: []string{"list"}},
		{name: "project prefix", args: []string{"proj:alpha", "list"}, wantProject: "alpha", wantProjectSet: true, wantRemaining: []string{"list"}},
		{name: "project prefix with empty name", args: []string{"proj:", "list"}, wantProject: "", wantProjectSet: true, wantRemaining: []string{"list"}},
		{name: "scope then project", args: []string{"na", "proj:alpha", "list"}, wantScope: taskScopeNoAgent, wantProject: "alpha", wantProjectSet: true, wantRemaining: []string{"list"}},
		{name: "project then scope", args: []string{"proj:alpha", "na", "list"}, wantScope: taskScopeNoAgent, wantProject: "alpha", wantProjectSet: true, wantRemaining: []string{"list"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotScope, gotProject, gotProjectSet, gotRemaining := parseTaskPrefixes(tc.args)
			if gotScope != tc.wantScope {
				t.Fatalf("scope = %v, want %v", gotScope, tc.wantScope)
			}
			if gotProject != tc.wantProject {
				t.Fatalf("project = %q, want %q", gotProject, tc.wantProject)
			}
			if gotProjectSet != tc.wantProjectSet {
				t.Fatalf("projectSet = %t, want %t", gotProjectSet, tc.wantProjectSet)
			}
			if !reflect.DeepEqual(gotRemaining, tc.wantRemaining) {
				t.Fatalf("remaining = %v, want %v", gotRemaining, tc.wantRemaining)
			}
		})
	}
}

func TestDispatcher_ProjectPrefix_PassesProjectOverride(t *testing.T) {
	var gotArgs []string
	var gotProject string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		gotArgs = append([]string(nil), args...)
		gotProject, _ = taskProjectFromContext(ctx)
		_, _ = io.WriteString(stdout, `[]`)
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.Dispatch(context.Background(), []string{"proj:alpha", "list"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("Dispatch code = %d, want 0", code)
	}
	if gotProject != "alpha" {
		t.Fatalf("project override = %q, want alpha", gotProject)
	}
	if !reflect.DeepEqual(gotArgs, []string{"status:pending", "export"}) {
		t.Fatalf("runner args = %v, want [status:pending export]", gotArgs)
	}
}

func TestDispatcher_NoAgentPrefix_StripsScopePrefix(t *testing.T) {
	taskJSONFor := func(uuid string) string {
		return `[{"uuid":"` + uuid + `","description":"Test","status":"pending","priority":"M","tags":[],"urgency":10,"depends":[]}]`
	}

	tests := []struct {
		name      string
		args      []string
		wantCalls [][]string
	}{
		{
			name:      "na defaults to list",
			args:      []string{"na"},
			wantCalls: [][]string{{"status:pending", "export"}},
		},
		{
			name:      "na list",
			args:      []string{"na", "list"},
			wantCalls: [][]string{{"status:pending", "export"}},
		},
		{
			name:      "no-agent info",
			args:      []string{"no-agent", "info", "test-uuid"},
			wantCalls: [][]string{{"uuid:test-uuid", "export"}},
		},
		{
			name:      "no-agent add",
			args:      []string{"no-agent", "add", "new task description"},
			wantCalls: [][]string{{"add", "rc.verbose=nothing", "rc.verbose=new-uuid", "new task description"}},
		},
		{
			name:      "na implicit add",
			args:      []string{"na", "description", "here"},
			wantCalls: [][]string{{"add", "rc.verbose=nothing", "rc.verbose=new-uuid", "description here"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var calls [][]string
			d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
				calls = append(calls, append([]string(nil), args...))
				switch strings.Join(args, " ") {
				case "status:pending export":
					_, _ = io.WriteString(stdout, taskJSONFor("test-uuid"))
				case "uuid:test-uuid export":
					_, _ = io.WriteString(stdout, taskJSONFor("test-uuid"))
				case "add rc.verbose=nothing rc.verbose=new-uuid new task description",
					"add rc.verbose=nothing rc.verbose=new-uuid description here":
					_, _ = io.WriteString(stdout, "Created task task-uuid-abc.\n")
				default:
					t.Fatalf("unexpected runner args: %v", args)
				}
				return 0, nil
			}})

			var stdout, stderr bytes.Buffer
			code, err := d.Dispatch(context.Background(), tc.args, nil, &stdout, &stderr)
			if err != nil {
				t.Fatalf("Dispatch returned error: %v", err)
			}
			if code != 0 {
				t.Fatalf("Dispatch code = %d, want 0", code)
			}
			if !reflect.DeepEqual(calls, tc.wantCalls) {
				t.Fatalf("runner calls = %#v, want %#v", calls, tc.wantCalls)
			}
		})
	}
}

func TestDispatcher_AllSubcommandsReachExecutor(t *testing.T) {
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC) }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	taskJSONFor := func(uuid string) string {
		return `[{"uuid":"` + uuid + `","description":"Test","status":"pending","priority":"M","tags":[],"urgency":10,"depends":[]}]`
	}

	cases := []struct {
		name      string
		args      []string
		wantCalls [][]string
	}{
		{
			name:      "list",
			args:      []string{"list"},
			wantCalls: [][]string{{"status:pending", "export"}},
		},
		{
			name:      "all",
			args:      []string{"all"},
			wantCalls: [][]string{{"export"}},
		},
		{
			name:      "ready",
			args:      []string{"ready"},
			wantCalls: [][]string{{"+READY", "export"}},
		},
		{
			name:      "urgency",
			args:      []string{"urgency"},
			wantCalls: [][]string{{"export"}},
		},
		{
			name:      "complete-uuids",
			args:      []string{"complete-uuids"},
			wantCalls: [][]string{{"status:pending", "export"}},
		},
		{
			name:      "complete-aliases",
			args:      []string{"complete-aliases"},
			wantCalls: [][]string{{"status:pending", "export"}},
		},
		{
			name:      "info",
			args:      []string{"info", "test-uuid"},
			wantCalls: [][]string{{"uuid:test-uuid", "export"}},
		},
		{
			name:      "add",
			args:      []string{"add", "new task description"},
			wantCalls: [][]string{{"add", "rc.verbose=nothing", "rc.verbose=new-uuid", "new task description"}},
		},
		{
			name:      "annotate",
			args:      []string{"annotate", "test-uuid", "note"},
			wantCalls: [][]string{{"uuid:test-uuid", "export"}, {"uuid:test-uuid", "annotate", "note"}},
		},
		{
			name:      "start",
			args:      []string{"start", "test-uuid"},
			wantCalls: [][]string{{"uuid:test-uuid", "export"}, {"uuid:test-uuid", "start"}},
		},
		{
			name:      "stop",
			args:      []string{"stop", "test-uuid"},
			wantCalls: [][]string{{"uuid:test-uuid", "export"}, {"uuid:test-uuid", "stop"}},
		},
		{
			name:      "done",
			args:      []string{"done", "test-uuid"},
			wantCalls: [][]string{{"uuid:test-uuid", "export"}, {"uuid:test-uuid", "done"}},
		},
		{
			name:      "priority",
			args:      []string{"priority", "test-uuid", "H"},
			wantCalls: [][]string{{"uuid:test-uuid", "export"}, {"uuid:test-uuid", "modify", "priority:H"}},
		},
		{
			name:      "tag",
			args:      []string{"tag", "test-uuid", "+cli"},
			wantCalls: [][]string{{"uuid:test-uuid", "export"}, {"uuid:test-uuid", "modify", "+cli"}},
		},
		{
			name:      "modify",
			args:      []string{"modify", "test-uuid", "priority:H"},
			wantCalls: [][]string{{"uuid:test-uuid", "export"}, {"uuid:test-uuid", "modify", "priority:H"}},
		},
		{
			name:      "denotate",
			args:      []string{"denotate", "test-uuid", "text"},
			wantCalls: [][]string{{"uuid:test-uuid", "export"}, {"uuid:test-uuid", "denotate", "text"}},
		},
		{
			name:      "delete",
			args:      []string{"delete", "test-uuid"},
			wantCalls: [][]string{{"uuid:test-uuid", "export"}, {"uuid:test-uuid", "delete"}},
		},
		{
			name:      "dep list",
			args:      []string{"dep", "list", "test-uuid"},
			wantCalls: [][]string{{"uuid:test-uuid", "export"}},
		},
		{
			name:      "dep add",
			args:      []string{"dep", "add", "test-uuid", "dep-uuid"},
			wantCalls: [][]string{{"uuid:test-uuid", "export"}, {"uuid:dep-uuid", "export"}, {"uuid:test-uuid", "modify", "depends:dep-uuid"}},
		},
		{
			name:      "dep rm",
			args:      []string{"dep", "rm", "test-uuid", "dep-uuid"},
			wantCalls: [][]string{{"uuid:test-uuid", "export"}, {"uuid:dep-uuid", "export"}, {"uuid:test-uuid", "modify", "depends:-dep-uuid"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls [][]string
			d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
				calls = append(calls, append([]string(nil), args...))
				switch strings.Join(args, " ") {
				case "export", "status:pending export", "+READY export":
					_, _ = io.WriteString(stdout, taskJSONFor("test-uuid"))
				case "uuid:test-uuid export":
					_, _ = io.WriteString(stdout, taskJSONFor("test-uuid"))
				case "uuid:dep-uuid export":
					_, _ = io.WriteString(stdout, taskJSONFor("dep-uuid"))
				case "add rc.verbose=nothing rc.verbose=new-uuid new task description":
					_, _ = io.WriteString(stdout, "Created task task-uuid-abc.\n")
				case "uuid:test-uuid annotate note":
				case "uuid:test-uuid start":
				case "uuid:test-uuid stop":
				case "uuid:test-uuid done":
				case "uuid:test-uuid modify priority:H":
				case "uuid:test-uuid modify +cli":
				case "uuid:test-uuid modify depends:dep-uuid":
				case "uuid:test-uuid modify depends:-dep-uuid":
				case "uuid:test-uuid denotate text":
				case "uuid:test-uuid delete":
				default:
					t.Fatalf("unexpected runner args: %v", args)
				}
				return 0, nil
			}})

			var stdout, stderr bytes.Buffer
			code, err := d.Dispatch(context.Background(), tc.args, nil, &stdout, &stderr)
			if err != nil {
				t.Fatalf("Dispatch returned error: %v", err)
			}
			if code != 0 {
				t.Fatalf("Dispatch code = %d, want 0", code)
			}
			if !reflect.DeepEqual(calls, tc.wantCalls) {
				t.Fatalf("runner calls = %#v, want %#v", calls, tc.wantCalls)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

type spyRunner struct {
	runFn func(context.Context, []string, io.Reader, io.Writer, io.Writer) (int, error)
}

func (s *spyRunner) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return s.runFn(ctx, args, stdin, stdout, stderr)
}
