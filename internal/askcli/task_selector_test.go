package askcli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveTaskSelectorFromCache_TouchesAliasEntry(t *testing.T) {
	dir := t.TempDir()
	oldNow := nowTaskAliasCache
	oldRoot := taskAliasCacheRoot
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	defer func() {
		nowTaskAliasCache = oldNow
		taskAliasCacheRoot = oldRoot
	}()

	writeTaskAliasCacheForTest(t, taskAliasCache{
		NextID: 2,
		Entries: []taskAliasCacheEntry{
			{
				UUID:           "task-uuid-1",
				Alias:          "0",
				CreatedAt:      time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC),
				LastAccessedAt: time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC),
			},
		},
	})

	resolved, err := resolveTaskSelectorFromCache("0", true)
	if err != nil {
		t.Fatalf("resolveTaskSelectorFromCache returned error: %v", err)
	}
	if !resolved.UsedAlias || resolved.UUID != "task-uuid-1" {
		t.Fatalf("resolved = %+v, want alias hit for task-uuid-1", resolved)
	}

	cache := readTaskAliasCacheSnapshot(t)
	entry := findTaskAliasEntry(t, cache, "task-uuid-1")
	if got := entry.LastAccessedAt; !got.Equal(nowTaskAliasCache()) {
		t.Fatalf("LastAccessedAt = %s, want %s", got, nowTaskAliasCache())
	}
}

func TestResolveTaskSelectorFromCache_MissingAlias(t *testing.T) {
	dir := t.TempDir()
	oldRoot := taskAliasCacheRoot
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	defer func() { taskAliasCacheRoot = oldRoot }()

	_, err := resolveTaskSelectorFromCache("a", true)
	if err == nil {
		t.Fatal("resolveTaskSelectorFromCache returned nil error, want missing alias failure")
	}
	if !strings.Contains(err.Error(), `did not match a known alias`) {
		t.Fatalf("err = %q, want missing alias message", err)
	}
}

func TestResolveTaskSelectorFromCache_AmbiguousSelector(t *testing.T) {
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
		NextID: 11,
		Entries: []taskAliasCacheEntry{
			{UUID: "a", Alias: "1", CreatedAt: nowTaskAliasCache()},
			{UUID: "task-uuid-2", Alias: "a", CreatedAt: nowTaskAliasCache()},
		},
	})

	_, err := resolveTaskSelectorFromCache("a", true)
	if err == nil {
		t.Fatal("resolveTaskSelectorFromCache returned nil error, want ambiguity")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("err = %q, want ambiguity message", err)
	}
}

func TestHandleInfo_StaleAlias(t *testing.T) {
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
			{UUID: "task-uuid-1", Alias: "0", CreatedAt: nowTaskAliasCache()},
		},
	})

	d := NewDispatcher(&spyRunner{runFn: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) == 2 && args[0] == "uuid:task-uuid-1" && args[1] == "export" {
			_, _ = io.WriteString(stdout, "[]")
		}
		return 0, nil
	}})

	var stdout, stderr bytes.Buffer
	code, _ := d.Dispatch(context.Background(), []string{"info", "0"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("info code = %d, want 1 for stale alias", code)
	}
	if !strings.Contains(stderr.String(), `alias "0" is stale`) {
		t.Fatalf("stderr = %q, want stale alias message", stderr.String())
	}
}

func writeTaskAliasCacheForTest(t *testing.T, cache taskAliasCache) {
	t.Helper()
	path, err := taskAliasCachePath()
	if err != nil {
		t.Fatalf("taskAliasCachePath: %v", err)
	}
	if err := cache.save(path); err != nil {
		t.Fatalf("cache.save: %v", err)
	}
}

func readTaskAliasCacheSnapshot(t *testing.T) taskAliasCache {
	t.Helper()
	path, err := taskAliasCachePath()
	if err != nil {
		t.Fatalf("taskAliasCachePath: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%s): %v", path, err)
	}
	var cache taskAliasCache
	if err := json.Unmarshal(data, &cache); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return cache
}
