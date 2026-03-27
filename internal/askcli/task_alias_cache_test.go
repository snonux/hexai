package askcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEncodeTaskAliasID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id   uint64
		want string
	}{
		{id: 0, want: "0"},
		{id: 9, want: "9"},
		{id: 10, want: "a"},
		{id: 35, want: "z"},
		{id: 36, want: "00"},
		{id: 37, want: "01"},
		{id: 71, want: "0z"},
		{id: 72, want: "10"},
		{id: 1331, want: "zz"},
		{id: 1332, want: "000"},
		{id: 1333, want: "001"},
	}

	for _, tc := range tests {
		if got := encodeTaskAliasID(tc.id); got != tc.want {
			t.Fatalf("encodeTaskAliasID(%d) = %q, want %q", tc.id, got, tc.want)
		}
	}
}

func TestDecodeTaskAliasID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		alias string
		want  uint64
		ok    bool
	}{
		{alias: "0", want: 0, ok: true},
		{alias: "z", want: 35, ok: true},
		{alias: "00", want: 36, ok: true},
		{alias: "01", want: 37, ok: true},
		{alias: "zz", want: 1331, ok: true},
		{alias: "000", want: 1332, ok: true},
		{alias: "", ok: false},
		{alias: "A", ok: false},
		{alias: "-", ok: false},
	}

	for _, tc := range tests {
		got, ok := decodeTaskAliasID(tc.alias)
		if ok != tc.ok {
			t.Fatalf("decodeTaskAliasID(%q) ok = %v, want %v", tc.alias, ok, tc.ok)
		}
		if ok && got != tc.want {
			t.Fatalf("decodeTaskAliasID(%q) = %d, want %d", tc.alias, got, tc.want)
		}
	}
}

func TestEnsureTaskAliases_PersistsAliasesAndTracksAccess(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	oldNow := nowTaskAliasCache
	oldRoot := taskAliasCacheRoot
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	defer func() {
		nowTaskAliasCache = oldNow
		taskAliasCacheRoot = oldRoot
	}()

	tasks := []TaskExport{{UUID: "uuid-1"}, {UUID: "uuid-2"}}
	aliases, err := ensureTaskAliases(tasks)
	if err != nil {
		t.Fatalf("ensureTaskAliases returned error: %v", err)
	}
	if aliases["uuid-1"] != "0" || aliases["uuid-2"] != "1" {
		t.Fatalf("aliases = %#v, want sequential aliases", aliases)
	}

	path, err := taskAliasCachePath()
	if err != nil {
		t.Fatalf("taskAliasCachePath: %v", err)
	}
	cache := readTaskAliasCacheForTest(t, path)
	if cache.NextID != 2 {
		t.Fatalf("NextID = %d, want 2", cache.NextID)
	}
	if len(cache.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(cache.Entries))
	}

	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC) }
	aliases, err = ensureTaskAliases([]TaskExport{{UUID: "uuid-2"}, {UUID: "uuid-3"}})
	if err != nil {
		t.Fatalf("ensureTaskAliases second call returned error: %v", err)
	}
	if aliases["uuid-2"] != "1" || aliases["uuid-3"] != "2" {
		t.Fatalf("second aliases = %#v, want existing+new aliases", aliases)
	}

	cache = readTaskAliasCacheForTest(t, path)
	if cache.NextID != 3 {
		t.Fatalf("NextID after second call = %d, want 3", cache.NextID)
	}
	entry := findTaskAliasEntry(t, cache, "uuid-2")
	if got := entry.LastAccessedAt; !got.Equal(nowTaskAliasCache()) {
		t.Fatalf("LastAccessedAt = %s, want %s", got, nowTaskAliasCache())
	}
}

func TestEnsureTaskAliases_PrunesExpiredEntriesWithoutReusingIDs(t *testing.T) {
	dir := t.TempDir()

	oldNow := nowTaskAliasCache
	oldRoot := taskAliasCacheRoot
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	defer func() {
		nowTaskAliasCache = oldNow
		taskAliasCacheRoot = oldRoot
	}()

	path, err := taskAliasCachePath()
	if err != nil {
		t.Fatalf("taskAliasCachePath: %v", err)
	}

	cache := taskAliasCache{
		NextID: 37,
		Entries: []taskAliasCacheEntry{
			{
				UUID:           "expired",
				Alias:          "z",
				CreatedAt:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				LastAccessedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			{
				UUID:           "fresh",
				Alias:          "00",
				CreatedAt:      time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
				LastAccessedAt: time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	if err := cache.save(path); err != nil {
		t.Fatalf("save seed cache: %v", err)
	}

	aliases, err := ensureTaskAliases([]TaskExport{{UUID: "fresh"}, {UUID: "new-task"}})
	if err != nil {
		t.Fatalf("ensureTaskAliases returned error: %v", err)
	}
	if aliases["fresh"] != "00" {
		t.Fatalf("fresh alias = %q, want 00", aliases["fresh"])
	}
	if aliases["new-task"] != "01" {
		t.Fatalf("new-task alias = %q, want 01", aliases["new-task"])
	}

	cache = readTaskAliasCacheForTest(t, path)
	if cache.NextID != 38 {
		t.Fatalf("NextID = %d, want 38", cache.NextID)
	}
	if len(cache.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2 after prune", len(cache.Entries))
	}
	if hasTaskAliasEntry(cache, "expired") {
		t.Fatalf("expired entry should have been pruned")
	}
}

func TestEnsureTaskAliases_DoesNotPruneEntriesAt120DayBoundary(t *testing.T) {
	dir := t.TempDir()

	oldNow := nowTaskAliasCache
	oldRoot := taskAliasCacheRoot
	nowTaskAliasCache = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	defer func() {
		nowTaskAliasCache = oldNow
		taskAliasCacheRoot = oldRoot
	}()

	path, err := taskAliasCachePath()
	if err != nil {
		t.Fatalf("taskAliasCachePath: %v", err)
	}

	boundary := nowTaskAliasCache().Add(-taskAliasCacheTTL)
	cache := taskAliasCache{
		NextID: 37,
		Entries: []taskAliasCacheEntry{
			{
				UUID:           "boundary",
				Alias:          "z",
				CreatedAt:      boundary,
				LastAccessedAt: boundary,
			},
		},
	}
	if err := cache.save(path); err != nil {
		t.Fatalf("save seed cache: %v", err)
	}

	aliases, err := ensureTaskAliases([]TaskExport{{UUID: "boundary"}})
	if err != nil {
		t.Fatalf("ensureTaskAliases returned error: %v", err)
	}
	if aliases["boundary"] != "z" {
		t.Fatalf("boundary alias = %q, want z", aliases["boundary"])
	}

	cache = readTaskAliasCacheForTest(t, path)
	if !hasTaskAliasEntry(cache, "boundary") {
		t.Fatal("boundary entry should not have been pruned")
	}
}

func TestEnsureTaskAliases_InvalidCacheReturnsError(t *testing.T) {
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
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ensureTaskAliases([]TaskExport{{UUID: "uuid-1"}}); err == nil {
		t.Fatal("expected error for invalid cache file")
	}
}

func TestEnsureTaskAliases_RejectsNextIDReuse(t *testing.T) {
	dir := t.TempDir()

	oldRoot := taskAliasCacheRoot
	taskAliasCacheRoot = func() (string, error) { return filepath.Join(dir, "hexai"), nil }
	defer func() { taskAliasCacheRoot = oldRoot }()

	path, err := taskAliasCachePath()
	if err != nil {
		t.Fatalf("taskAliasCachePath: %v", err)
	}

	cache := taskAliasCache{
		NextID: 36,
		Entries: []taskAliasCacheEntry{
			{UUID: "uuid-1", Alias: "00", CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		},
	}
	if err := cache.save(path); err != nil {
		t.Fatalf("save seed cache: %v", err)
	}

	if _, err := ensureTaskAliases([]TaskExport{{UUID: "uuid-2"}}); err == nil {
		t.Fatal("expected error when next_id would reuse an alias")
	}
}

func readTaskAliasCacheForTest(t *testing.T, path string) taskAliasCache {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	var cache taskAliasCache
	if err := json.Unmarshal(data, &cache); err != nil {
		t.Fatalf("Unmarshal(%s): %v", path, err)
	}
	return cache
}

func findTaskAliasEntry(t *testing.T, cache taskAliasCache, uuid string) taskAliasCacheEntry {
	t.Helper()

	for _, entry := range cache.Entries {
		if entry.UUID == uuid {
			return entry
		}
	}
	t.Fatalf("missing alias entry for %q", uuid)
	return taskAliasCacheEntry{}
}

func hasTaskAliasEntry(cache taskAliasCache, uuid string) bool {
	for _, entry := range cache.Entries {
		if entry.UUID == uuid {
			return true
		}
	}
	return false
}
