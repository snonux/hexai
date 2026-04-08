package askcli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleCompleteUUIDs_PrintsPendingUUIDs(t *testing.T) {
	dir := t.TempDir()
	oldNow := nowTaskAliasCache
	oldRoot := taskAliasCacheRoot
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	defer func() {
		nowTaskAliasCache = oldNow
		taskAliasCacheRoot = oldRoot
	}()

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		want := []string{"status:pending", "export"}
		if strings.Join(args, " ") != strings.Join(want, " ") {
			t.Fatalf("args = %v, want %v", args, want)
		}
		_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1","description":"First task"},{"uuid":"uuid-2","description":"Second task"},{"uuid":""}]`)
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.handleCompleteUUIDs(context.Background(), nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("handleCompleteUUIDs returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("handleCompleteUUIDs code = %d, want 0", code)
	}
	// Each line is "selector\tdescription" so fish shell shows the task
	// summary alongside the alias/UUID in the autocompletion menu.
	if got := stdout.String(); got != "0\tFirst task\nuuid-1\tFirst task\n1\tSecond task\nuuid-2\tSecond task\n" {
		t.Fatalf("stdout = %q, want tab-separated selector+description list", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	path, err := taskAliasCachePath()
	if err != nil {
		t.Fatalf("taskAliasCachePath: %v", err)
	}
	cache := readTaskAliasCacheForTest(t, path)
	if got := findTaskAliasEntry(t, cache, "uuid-1").Alias; got != "0" {
		t.Fatalf("uuid-1 alias = %q, want 0", got)
	}
	if got := findTaskAliasEntry(t, cache, "uuid-2").Alias; got != "1" {
		t.Fatalf("uuid-2 alias = %q, want 1", got)
	}
}

func TestHandleCompleteUUIDs_ParseError(t *testing.T) {
	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stdout, `not-json`)
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.handleCompleteUUIDs(context.Background(), nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("handleCompleteUUIDs returned error: %v", err)
	}
	if code != 1 {
		t.Fatalf("handleCompleteUUIDs code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "failed to parse task data") {
		t.Fatalf("stderr = %q, want parse error", stderr.String())
	}
}

func TestHandleCompleteUUIDs_RecoverFromCorruptAliasCache(t *testing.T) {
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	defer func() { taskAliasCacheRoot = oldRoot }()

	path, err := taskAliasCachePath()
	if err != nil {
		t.Fatalf("taskAliasCachePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Simulate a corrupted cache file (e.g. two JSON objects concatenated from a
	// concurrent write race). The handler must recover by resetting the cache and
	// assigning fresh aliases rather than erroring or degrading to UUID-only output.
	if err := os.WriteFile(path, []byte("{bad"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1","description":"Fallback task"}]`)
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.handleCompleteUUIDs(context.Background(), nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("handleCompleteUUIDs returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("handleCompleteUUIDs code = %d, want 0", code)
	}
	// After recovery a fresh alias (e.g. "0") must be assigned, so the output
	// includes both the short alias and the UUID (fish shows whichever the user
	// types). No warning should appear on stderr.
	got := stdout.String()
	if !strings.Contains(got, "uuid-1\tFallback task") {
		t.Fatalf("stdout = %q, want UUID with description", got)
	}
	if !strings.Contains(got, "Fallback task") {
		t.Fatalf("stdout = %q, want task description in output", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want no warnings after graceful recovery", stderr.String())
	}
}

func TestTaskCompletionSelectors_SkipsMissingAndDuplicateAliases(t *testing.T) {
	tasks := []TaskExport{
		{UUID: "uuid-1"},
		{UUID: ""},
		{UUID: "uuid-2"},
	}
	aliases := map[string]string{
		"uuid-1": "0",
		"uuid-2": "uuid-2",
	}

	got := taskCompletionSelectors(tasks, aliases)
	want := []string{"0", "uuid-1", "uuid-2"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("taskCompletionSelectors = %v, want %v", got, want)
	}
}

func TestTaskCompletionAliasItems_OnlyShortAliases(t *testing.T) {
	tasks := []TaskExport{
		{UUID: "uuid-1", Description: "First task"},
		{UUID: "", Description: "Ignored"},
		{UUID: "uuid-2", Description: "Second task"},
	}
	aliases := map[string]string{
		"uuid-1": "0",
		"uuid-2": "uuid-2", // same as UUID: no alias-only line (use complete-uuids for UUID)
	}

	got := taskCompletionAliasItems(tasks, aliases)
	want := []string{
		"0\tFirst task",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("taskCompletionAliasItems = %v, want %v", got, want)
	}
}

func TestHandleCompleteAliases_PrintsAliasesOnly(t *testing.T) {
	dir := t.TempDir()
	oldNow := nowTaskAliasCache
	oldRoot := taskAliasCacheRoot
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	defer func() {
		nowTaskAliasCache = oldNow
		taskAliasCacheRoot = oldRoot
	}()

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		want := []string{"status:pending", "export"}
		if strings.Join(args, " ") != strings.Join(want, " ") {
			t.Fatalf("args = %v, want %v", args, want)
		}
		_, _ = io.WriteString(stdout, `[{"uuid":"uuid-1","description":"First task"},{"uuid":"uuid-2","description":"Second task"},{"uuid":""}]`)
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, err := d.handleCompleteAliases(context.Background(), nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("handleCompleteAliases returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("handleCompleteAliases code = %d, want 0", code)
	}
	if got := stdout.String(); got != "0\tFirst task\n1\tSecond task\n" {
		t.Fatalf("stdout = %q, want alias-only tab-separated list", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestTaskCompletionItems_IncludesDescriptions(t *testing.T) {
	tasks := []TaskExport{
		{UUID: "uuid-1", Description: "First task"},
		{UUID: "", Description: "Ignored"},
		{UUID: "uuid-2", Description: "Second task"},
	}
	aliases := map[string]string{
		"uuid-1": "0",
		"uuid-2": "uuid-2", // same as UUID, so alias is skipped
	}

	got := taskCompletionItems(tasks, aliases)
	// Alias "0" differs from UUID so it gets its own entry; "uuid-2" matches
	// UUID so no alias entry is emitted for it.
	want := []string{
		"0\tFirst task",
		"uuid-1\tFirst task",
		"uuid-2\tSecond task",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("taskCompletionItems = %v, want %v", got, want)
	}
}

func TestTruncateDescription_ShortString(t *testing.T) {
	got := truncateDescription("hello", 10)
	if got != "hello" {
		t.Fatalf("truncateDescription = %q, want %q", got, "hello")
	}
}

func TestTruncateDescription_ExactLength(t *testing.T) {
	got := truncateDescription("hello", 5)
	if got != "hello" {
		t.Fatalf("truncateDescription = %q, want %q", got, "hello")
	}
}

func TestTruncateDescription_LongString(t *testing.T) {
	got := truncateDescription("hello world", 5)
	if got != "hello…" {
		t.Fatalf("truncateDescription = %q, want %q", got, "hello…")
	}
}

func TestTruncateDescription_Unicode(t *testing.T) {
	// Japanese characters are multi-byte but each is one rune, so maxLen=3
	// should cut at 3 runes.
	got := truncateDescription("日本語テスト", 3)
	if got != "日本語…" {
		t.Fatalf("truncateDescription = %q, want %q", got, "日本語…")
	}
}
