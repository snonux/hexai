package askcli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleInfo_Success(t *testing.T) {
	unsetTestEnv(t, "HEXAI_DEBUG")
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 2,
		Entries: []taskAliasCacheEntry{
			{UUID: "dep-1", Alias: "1", CreatedAt: nowTaskAliasCache()},
			{UUID: "test-uuid", Alias: "0", CreatedAt: nowTaskAliasCache()},
		},
	})

	jsonData := `[{"uuid":"test-uuid","description":"Test task","status":"pending","priority":"H","tags":["cli","agent"],"urgency":15.0,"depends":["dep-1"],"annotations":[{"description":"Note 1","entry":"2026-03-22T10:00:00Z"}]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		// args[0] is "uuid:<uuid>" (the filter); emit data for any export call
		if len(args) > 0 && strings.HasPrefix(args[0], "uuid:") {
			_, _ = io.WriteString(stdout, jsonData)
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"info", "test-uuid"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("info code = %d, want 0", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "ID:          0") {
		t.Fatalf("output missing alias ID: %s", output)
	}
	if strings.Contains(output, "UUID:") || strings.Contains(output, "test-uuid") {
		t.Fatalf("output leaked UUID in default mode: %s", output)
	}
	if !strings.Contains(output, "H") {
		t.Fatalf("output missing priority: %s", output)
	}
	if !strings.Contains(output, "Started:     no") {
		t.Fatalf("output missing explicit started state: %s", output)
	}
	if !strings.Contains(output, "Depends:     1 (dep-1)") {
		t.Fatalf("output missing formatted dependency alias: %s", output)
	}
}

func TestHandleInfo_Success_DebugShowsUUID(t *testing.T) {
	t.Setenv("HEXAI_DEBUG", "true")
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "test-uuid", Alias: "0", CreatedAt: nowTaskAliasCache()},
		},
	})

	jsonData := `[{"uuid":"test-uuid","description":"Test task","status":"pending","priority":"H","tags":["cli","agent"],"urgency":15.0,"depends":[],"annotations":[]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) > 0 && strings.HasPrefix(args[0], "uuid:") {
			_, _ = io.WriteString(stdout, jsonData)
		}
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"info", "test-uuid"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("info code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "UUID:        test-uuid") {
		t.Fatalf("output missing UUID in debug mode: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "ID:          0") {
		t.Fatalf("output missing alias ID in debug mode: %s", stdout.String())
	}
}

func TestHandleInfo_AssignsDependencyAliasesFromInfo(t *testing.T) {
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "test-uuid", Alias: "0", CreatedAt: nowTaskAliasCache()},
		},
	})

	jsonData := `[{"uuid":"test-uuid","description":"Test task","status":"pending","priority":"H","tags":["cli"],"urgency":15.0,"depends":["dep-b","dep-a"]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) > 0 && strings.HasPrefix(args[0], "uuid:") {
			_, _ = io.WriteString(stdout, jsonData)
		}
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"info", "test-uuid"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("info code = %d, want 0", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "Depends:") ||
		!strings.Contains(output, "(dep-a)") ||
		!strings.Contains(output, "(dep-b)") {
		t.Fatalf("output missing assigned dependency aliases: %s", output)
	}
}

func TestHandleInfo_AliasSelector(t *testing.T) {
	unsetTestEnv(t, "HEXAI_DEBUG")
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "test-uuid", Alias: "0", CreatedAt: nowTaskAliasCache()},
		},
	})

	jsonData := `[{"uuid":"test-uuid","description":"Test task","status":"pending","priority":"H","tags":["cli"],"urgency":15.0,"depends":[]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) > 0 && strings.HasPrefix(args[0], "uuid:test-uuid") {
			_, _ = io.WriteString(stdout, jsonData)
		}
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"info", "0"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("info code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "ID:          0") {
		t.Fatalf("stdout = %q, want alias ID", stdout.String())
	}
	if strings.Contains(stdout.String(), "UUID:") || strings.Contains(stdout.String(), "test-uuid") {
		t.Fatalf("stdout = %q, want UUID hidden by default", stdout.String())
	}
}

func TestHandleInfo_JSONIncludesAliasID(t *testing.T) {
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "test-uuid", Alias: "0", CreatedAt: nowTaskAliasCache()},
		},
	})

	jsonData := `[{"uuid":"test-uuid","description":"Test task","status":"pending","priority":"H","tags":["cli"],"urgency":15.0,"depends":[]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) > 0 && strings.HasPrefix(args[0], "uuid:test-uuid") {
			_, _ = io.WriteString(stdout, jsonData)
		}
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"--json", "info", "0"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("info code = %d, want 0", code)
	}

	var parsed []map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &parsed); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("parsed len = %d, want 1", len(parsed))
	}
	if got := parsed[0]["id"]; got != "0" {
		t.Fatalf("json id = %#v, want 0", got)
	}
	if got := parsed[0]["uuid"]; got != "test-uuid" {
		t.Fatalf("json uuid = %#v, want test-uuid", got)
	}
}

func TestHandleInfo_NumericID(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"info", "123"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("info code = %d, want 1 for numeric ID", code)
	}
}

func TestHandleInfo_MissingUUID(t *testing.T) {
	unsetTestEnv(t, "HEXAI_DEBUG")
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	oldNow := nowTaskAliasCache
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	defer func() {
		taskAliasCacheRoot = oldRoot
		nowTaskAliasCache = oldNow
	}()

	jsonData := `[{"uuid":"started-uuid","description":"Started task","status":"pending","priority":"M","start":"2026-03-26T10:00:00Z","urgency":5.0,"depends":[]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "started" && args[1] == "export" {
			_, _ = io.WriteString(stdout, jsonData)
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"info"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("info code = %d, want 0 for implicit started task", code)
	}
	if !strings.Contains(stdout.String(), "ID:          0") {
		t.Fatalf("output missing alias: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "UUID:") || strings.Contains(stdout.String(), "started-uuid") {
		t.Fatalf("output leaked started task UUID in default mode: %s", stdout.String())
	}
}

func TestHandleInfo_MissingUUID_NoStartedTask(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "started" && args[1] == "export" {
			_, _ = io.WriteString(stdout, "[]")
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"info"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("info code = %d, want 1 when no started task exists", code)
	}
	if !strings.Contains(stderr.String(), "no started task found") {
		t.Fatalf("stderr = %q, want no-started-task error", stderr.String())
	}
}

func TestHandleInfo_MissingUUID_MultipleStartedTasks(t *testing.T) {
	jsonData := `[{"uuid":"started-1","description":"Started task 1","status":"pending","priority":"M","start":"2026-03-26T10:00:00Z","urgency":5.0,"depends":[]},{"uuid":"started-2","description":"Started task 2","status":"pending","priority":"H","start":"2026-03-26T11:00:00Z","urgency":8.0,"depends":[]}]`
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "started" && args[1] == "export" {
			_, _ = io.WriteString(stdout, jsonData)
		}
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"info"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("info code = %d, want 1 when multiple started tasks exist", code)
	}
	if !strings.Contains(stderr.String(), "multiple started tasks found") {
		t.Fatalf("stderr = %q, want multiple-started-tasks error", stderr.String())
	}
	if !strings.Contains(stderr.String(), "ID or UUID explicitly") {
		t.Fatalf("stderr = %q, want ID-or-UUID guidance", stderr.String())
	}
}

func TestHandleAdd_Success(t *testing.T) {
	now := useIsolatedTaskAliasCache(t)
	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 1,
		Entries: []taskAliasCacheEntry{
			{UUID: "existing-uuid", Alias: "0", CreatedAt: now},
		},
	})

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stdout, "Created task abc-123-def.")
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"add", "New task description"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("add code = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "created task 1" {
		t.Fatalf("stdout = %q, want created task 1", stdout.String())
	}
	cache := readTaskAliasCacheSnapshot(t)
	entry := findTaskAliasEntry(t, cache, "abc-123-def")
	if entry.Alias != "1" {
		t.Fatalf("created task alias = %q, want 1", entry.Alias)
	}
}

func TestHandleAdd_AliasAssignmentFailure(t *testing.T) {
	oldRoot := taskAliasCacheRoot
	taskAliasCacheRoot = func() (string, error) { return "", io.ErrUnexpectedEOF }
	defer func() { taskAliasCacheRoot = oldRoot }()

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stdout, "Created task abc-123-def.")
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"add", "New task description"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("add code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty output on alias assignment failure", stdout.String())
	}
	if !strings.Contains(stderr.String(), "failed to assign task alias") {
		t.Fatalf("stderr = %q, want alias assignment failure", stderr.String())
	}
}

func TestHandleAdd_MissingDescription(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}})
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"add"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("add code = %d, want 1 for missing description", code)
	}
}

func TestHandleAdd_DependsModifierWithoutSelectors(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		t.Fatalf("runner should not be called when depends: has no selectors: %v", args)
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"add", "depends:", "New", "task"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("add code = %d, want 1", code)
	}
	if got := stderr.String(); !strings.Contains(got, "do add depends:<id|uuid>[,<id|uuid>...] requires at least one dependency ID or UUID") {
		t.Fatalf("stderr = %q, want depends: selector error", got)
	}
}

func makeAddRunner(onAdd func(args []string, stdout io.Writer)) *spyRunner {
	return &spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		onAdd(args, stdout)
		return 0, nil
	}}
}

func TestHandleAdd_MultipleWords(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(makeAddRunner(func(args []string, stdout io.Writer) {
		capturedArgs = args
		_, _ = io.WriteString(stdout, "Created task test-uuid.")
	}))
	var stdout, stderr bytes.Buffer
	d.Dispatch(context.Background(), []string{"add", "Multi", "word", "description"}, nil, &stdout, &stderr)
	// args[0]="add", args[1]="rc.verbose=nothing", args[2]="rc.verbose=new-uuid"
	if len(capturedArgs) < 4 || capturedArgs[0] != "add" || capturedArgs[1] != "rc.verbose=nothing" || capturedArgs[2] != "rc.verbose=new-uuid" {
		t.Fatalf("capturedArgs = %v, want [add, rc.verbose=nothing, rc.verbose=new-uuid, ...]", capturedArgs)
	}
}

func TestHandleAdd_WithPriority(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(makeAddRunner(func(args []string, stdout io.Writer) {
		capturedArgs = args
		_, _ = io.WriteString(stdout, "Created task test-uuid.")
	}))
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"add", "priority:H", "Fix critical bug"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("add code = %d, want 0", code)
	}
	// args: [add, rc.verbose=nothing, rc.verbose=new-uuid, priority:H, Fix critical bug]
	if len(capturedArgs) < 5 {
		t.Fatalf("capturedArgs = %v, want at least 5 elements", capturedArgs)
	}
	if capturedArgs[3] != "priority:H" {
		t.Errorf("capturedArgs[3] = %s, want priority:H", capturedArgs[3])
	}
	if capturedArgs[len(capturedArgs)-1] != "Fix critical bug" {
		t.Errorf("last arg = %s, want 'Fix critical bug'", capturedArgs[len(capturedArgs)-1])
	}
}

func TestHandleAdd_WithTag(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(makeAddRunner(func(args []string, stdout io.Writer) {
		capturedArgs = args
		_, _ = io.WriteString(stdout, "Created task test-uuid.")
	}))
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"add", "+cli", "New feature"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("add code = %d, want 0", code)
	}
	// args: [add, rc.verbose=nothing, rc.verbose=new-uuid, +cli, New feature]
	if capturedArgs[3] != "+cli" {
		t.Errorf("capturedArgs[3] = %s, want +cli", capturedArgs[3])
	}
}

func TestHandleAdd_WithPriorityAndTag(t *testing.T) {
	var capturedArgs []string
	d := NewDispatcher(makeAddRunner(func(args []string, stdout io.Writer) {
		capturedArgs = args
		_, _ = io.WriteString(stdout, "Created task test-uuid.")
	}))
	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"add", "priority:H", "+cli", "Complex task"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("add code = %d, want 0", code)
	}
	// args: [add, rc.verbose=nothing, rc.verbose=new-uuid, priority:H, +cli, Complex task]
	if capturedArgs[3] != "priority:H" || capturedArgs[4] != "+cli" {
		t.Errorf("capturedArgs = %v, want [add, rc.verbose=nothing, rc.verbose=new-uuid, priority:H, +cli, Complex task]", capturedArgs)
	}
}

func TestHandleAdd_WithDependencies(t *testing.T) {
	now := useIsolatedTaskAliasCache(t)
	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 2,
		Entries: []taskAliasCacheEntry{
			{UUID: "dep-uuid-1", Alias: "0", CreatedAt: now},
			{UUID: "dep-uuid-2", Alias: "1", CreatedAt: now},
		},
	})

	var capturedAddArgs []string
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		switch {
		case len(args) >= 2 && args[0] == "uuid:dep-uuid-1" && args[1] == "export":
			_, _ = io.WriteString(stdout, `[{"uuid":"dep-uuid-1","description":"Dependency one","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
		case len(args) >= 2 && args[0] == "uuid:dep-uuid-2" && args[1] == "export":
			_, _ = io.WriteString(stdout, `[{"uuid":"dep-uuid-2","description":"Dependency two","status":"pending","priority":"M","tags":[],"urgency":0,"depends":[]}]`)
		case len(args) >= 1 && args[0] == "add":
			capturedAddArgs = append([]string(nil), args...)
			_, _ = io.WriteString(stdout, "Created task created-uuid.")
		default:
			t.Fatalf("unexpected runner args: %v", args)
		}
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(
		context.Background(),
		[]string{"add", "+cli", "depends:0,1", "New", "task"},
		nil,
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("add code = %d, want 0: stderr=%s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "created task 2" {
		t.Fatalf("stdout = %q, want created task 2", stdout.String())
	}
	if len(capturedAddArgs) < 6 {
		t.Fatalf("capturedAddArgs = %v, want add invocation with dependency modifier", capturedAddArgs)
	}
	if capturedAddArgs[3] != "+cli" {
		t.Fatalf("capturedAddArgs[3] = %q, want +cli", capturedAddArgs[3])
	}
	if capturedAddArgs[4] != "depends:dep-uuid-1,dep-uuid-2" {
		t.Fatalf("capturedAddArgs[4] = %q, want dependency modifier", capturedAddArgs[4])
	}
	if capturedAddArgs[5] != "New task" {
		t.Fatalf("capturedAddArgs[5] = %q, want joined description", capturedAddArgs[5])
	}
}

func TestExtractUUIDFromAddOutput(t *testing.T) {
	if uuid := extractUUIDFromAddOutput("Created task abc-123-def."); uuid != "abc-123-def" {
		t.Fatalf("got %q, want abc-123-def", uuid)
	}
	if uuid := extractUUIDFromAddOutput("Created task abc-123-def.\nsome other line"); uuid != "abc-123-def" {
		t.Fatalf("got %q, want abc-123-def", uuid)
	}
	if uuid := extractUUIDFromAddOutput("no match here"); uuid != "" {
		t.Fatalf("got %q, want empty", uuid)
	}
}

func TestParseAddArgs(t *testing.T) {
	mods, desc, deps, err := parseAddArgs([]string{"priority:H", "+cli", "Fix bug"})
	if err != nil || desc != "Fix bug" || len(mods) != 2 || len(deps) != 0 {
		t.Fatalf("parseAddArgs([\"priority:H\", \"+cli\", \"Fix bug\"]) = mods=%v, desc=%q, deps=%v, err=%v", mods, desc, deps, err)
	}

	mods, desc, deps, err = parseAddArgs([]string{"Multi", "word", "description"})
	if err != nil || desc != "Multi word description" || len(mods) != 0 || len(deps) != 0 {
		t.Fatalf("parseAddArgs([\"Multi\", \"word\", \"description\"]) = mods=%v, desc=%q, deps=%v, err=%v", mods, desc, deps, err)
	}

	mods, desc, deps, err = parseAddArgs([]string{"-deprecated", "Old task"})
	if err != nil || desc != "Old task" || len(mods) != 1 || mods[0] != "-deprecated" || len(deps) != 0 {
		t.Fatalf("parseAddArgs([\"-deprecated\", \"Old task\"]) = mods=%v, desc=%q, deps=%v, err=%v", mods, desc, deps, err)
	}

	// An arg starting with "+" but containing spaces is NOT a modifier — it is
	// the start of the description. This prevents agents from quoting tag+desc
	// together (e.g. "+code-quality Fix foo") and having them land in the wrong
	// place.
	mods, desc, deps, err = parseAddArgs([]string{"+code-quality Fix foo bar"})
	if err != nil || desc != "+code-quality Fix foo bar" || len(mods) != 0 || len(deps) != 0 {
		t.Fatalf("space-containing +arg should be description, got mods=%v, desc=%q, deps=%v, err=%v", mods, desc, deps, err)
	}

	// Same issue when mixed: a proper tag precedes a space-containing arg.
	mods, desc, deps, err = parseAddArgs([]string{"+cli", "+code-quality Fix foo bar"})
	if err != nil || desc != "+code-quality Fix foo bar" || len(mods) != 1 || mods[0] != "+cli" || len(deps) != 0 {
		t.Fatalf("mixed case: mods=%v, desc=%q, deps=%v, err=%v", mods, desc, deps, err)
	}

	// All-modifier args (no description) should return empty description, not a
	// duplicate of the modifiers.
	mods, desc, deps, err = parseAddArgs([]string{"+cli", "+agent"})
	if err != nil || desc != "" || len(mods) != 2 || len(deps) != 0 {
		t.Fatalf("all-modifier case: mods=%v, desc=%q, deps=%v, err=%v", mods, desc, deps, err)
	}

	mods, desc, deps, err = parseAddArgs([]string{"+cli", "depends:0,1", "Fix", "bug"})
	if err != nil || desc != "Fix bug" || len(mods) != 1 || mods[0] != "+cli" || len(deps) != 2 || deps[0] != "0" || deps[1] != "1" {
		t.Fatalf("depends case: mods=%v, desc=%q, deps=%v, err=%v", mods, desc, deps, err)
	}

	if _, _, _, err = parseAddArgs([]string{"depends:", "Fix", "bug"}); err == nil {
		t.Fatalf("parseAddArgs should reject empty depends: modifier")
	}
	if _, _, _, err = parseAddArgs([]string{"depends:0,,1", "Fix", "bug"}); err == nil {
		t.Fatalf("parseAddArgs should reject empty selector entries")
	}
}
