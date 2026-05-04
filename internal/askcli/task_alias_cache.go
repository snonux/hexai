package askcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"codeberg.org/snonux/hexai/internal/filelock"
	"codeberg.org/snonux/hexai/internal/stats"
)

const (
	taskAliasCacheTTL          = 120 * 24 * time.Hour
	taskAliasCacheLockFileName = "task-aliases-v2.json.lock"
	taskAliasCacheLockTimeout  = 30 * time.Second
)

var (
	nowTaskAliasCache  = time.Now
	taskAliasCacheRoot = stats.CacheDir
)

// taskAliasCache maps UUIDs to short human-readable aliases for display and
// completion. The Entries slice is persisted as JSON; the byUUID and byAlias
// maps are derived in-memory for O(1) lookups and are rebuilt after every load
// or mutation.
type taskAliasCache struct {
	NextID  uint64                `json:"next_id"`
	Entries []taskAliasCacheEntry `json:"entries"`

	// In-memory lookup maps — not serialized to JSON.
	byUUID  map[string]*taskAliasCacheEntry
	byAlias map[string]*taskAliasCacheEntry
}

type taskAliasCacheEntry struct {
	UUID           string    `json:"uuid"`
	Alias          string    `json:"alias"`
	CreatedAt      time.Time `json:"created_at"`
	LastAccessedAt time.Time `json:"last_accessed_at"`
}

func ensureTaskAliases(tasks []TaskExport) (map[string]string, error) {
	path, err := taskAliasCachePath()
	if err != nil {
		return nil, err
	}
	// Hold an advisory lock for the full load/modify/save cycle so concurrent
	// ask invocations cannot race on the cache file (lost-update or
	// rename-against-missing-tempfile).
	unlock, err := acquireTaskAliasCacheLock(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = unlock() }()

	cache, err := loadTaskAliasCacheAt(path)
	if err != nil {
		return nil, err
	}

	now := nowTaskAliasCache().UTC()
	changed := cache.prune(now)
	aliases := make(map[string]string, len(tasks))
	for _, task := range tasks {
		if task.UUID == "" {
			continue
		}
		alias, updated := cache.ensureAlias(task.UUID, now)
		aliases[task.UUID] = alias
		changed = changed || updated
	}

	if !changed {
		return aliases, nil
	}
	if err := cache.save(path); err != nil {
		return nil, err
	}
	return aliases, nil
}

func ensureTaskAliasesForUUIDs(uuids []string) (map[string]string, error) {
	tasks := make([]TaskExport, 0, len(uuids))
	for _, uuid := range uuids {
		if uuid == "" {
			continue
		}
		tasks = append(tasks, TaskExport{UUID: uuid})
	}
	return ensureTaskAliases(tasks)
}

func loadTaskAliasCacheAt(path string) (taskAliasCache, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return taskAliasCache{}, nil
	}
	if err != nil {
		return taskAliasCache{}, fmt.Errorf("read task alias cache: %w", err)
	}

	var cache taskAliasCache
	if err := json.Unmarshal(data, &cache); err != nil {
		// Cache file is unreadable (e.g. truncated write or duplicate-write
		// corruption). Discard and start fresh — tasks will get new alias IDs on
		// the next run, which is preferable to a hard failure.
		_ = os.Remove(path)
		return taskAliasCache{}, nil
	}
	if err := cache.validate(); err != nil {
		return taskAliasCache{}, fmt.Errorf("validate task alias cache: %w", err)
	}
	// Rebuild lookup maps from the deserialized entries so that all subsequent
	// operations use O(1) map access rather than a linear scan.
	cache.rebuildMaps()
	return cache, nil
}

// acquireTaskAliasCacheLock takes an exclusive advisory lock on a sentinel
// file in dir so that concurrent ask invocations serialize their
// load/modify/save of the alias cache. The returned closure releases the lock
// and closes the underlying file.
func acquireTaskAliasCacheLock(dir string) (func() error, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create task alias cache dir: %w", err)
	}
	lockPath := filepath.Join(dir, taskAliasCacheLockFileName)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open task alias cache lock: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), taskAliasCacheLockTimeout)
	release, err := filelock.AcquireExclusive(ctx, f)
	cancel()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock task alias cache: %w", err)
	}
	return func() error {
		rerr := release()
		cerr := f.Close()
		return errors.Join(rerr, cerr)
	}, nil
}

func taskAliasCachePath() (string, error) {
	dir, err := taskAliasCacheRoot()
	if err != nil {
		return "", fmt.Errorf("resolve cache dir: %w", err)
	}
	// v2 uses reversed alias strings (e.g. "10" instead of "01") so that the
	// first character varies more often, improving shell auto-completion. The
	// old v1 file is intentionally abandoned so the mapping starts fresh.
	return filepath.Join(dir, "ask", "task-aliases-v2.json"), nil
}

func (c *taskAliasCache) validate() error {
	seenUUIDs := make(map[string]struct{}, len(c.Entries))
	seenAliases := make(map[string]struct{}, len(c.Entries))
	var maxID uint64
	hasEntries := false
	for _, entry := range c.Entries {
		if entry.UUID == "" {
			return fmt.Errorf("entry missing uuid")
		}
		if entry.Alias == "" {
			return fmt.Errorf("entry %q missing alias", entry.UUID)
		}
		id, ok := decodeTaskAliasID(entry.Alias)
		if !ok {
			return fmt.Errorf("entry %q has invalid alias %q", entry.UUID, entry.Alias)
		}
		if _, ok := seenUUIDs[entry.UUID]; ok {
			return fmt.Errorf("duplicate uuid %q", entry.UUID)
		}
		if _, ok := seenAliases[entry.Alias]; ok {
			return fmt.Errorf("duplicate alias %q", entry.Alias)
		}
		seenUUIDs[entry.UUID] = struct{}{}
		seenAliases[entry.Alias] = struct{}{}
		if !hasEntries || id > maxID {
			maxID = id
			hasEntries = true
		}
	}
	if hasEntries && c.NextID <= maxID {
		return fmt.Errorf("next_id %d must be greater than max alias id %d", c.NextID, maxID)
	}
	return nil
}

// rebuildMaps repopulates byUUID and byAlias from the current Entries slice.
// This must be called after any operation that mutates Entries (load, prune,
// add). Pointer stability is guaranteed because entries are addressed by their
// slice index at the time the map is built; both maps are fully rebuilt each
// time rather than patched to avoid stale pointers after slice reallocation.
func (c *taskAliasCache) rebuildMaps() {
	c.byUUID = make(map[string]*taskAliasCacheEntry, len(c.Entries))
	c.byAlias = make(map[string]*taskAliasCacheEntry, len(c.Entries))
	for i := range c.Entries {
		c.byUUID[c.Entries[i].UUID] = &c.Entries[i]
		c.byAlias[c.Entries[i].Alias] = &c.Entries[i]
	}
}

func (c *taskAliasCache) prune(now time.Time) bool {
	if len(c.Entries) == 0 {
		return false
	}

	kept := c.Entries[:0]
	changed := false
	for _, entry := range c.Entries {
		if now.Sub(entry.lastTouchedAt()) > taskAliasCacheTTL {
			changed = true
			continue
		}
		kept = append(kept, entry)
	}
	c.Entries = kept
	if changed {
		c.sortEntries()
		// Rebuild maps after pruning so removed entries are no longer reachable.
		c.rebuildMaps()
	}
	return changed
}

// ensureAlias returns the alias for uuid, creating one if necessary. It uses
// the byUUID map for O(1) lookup instead of a linear slice scan.
func (c *taskAliasCache) ensureAlias(uuid string, now time.Time) (string, bool) {
	if entry, ok := c.byUUID[uuid]; ok {
		if entry.LastAccessedAt.Equal(now) {
			return entry.Alias, false
		}
		entry.LastAccessedAt = now
		return entry.Alias, true
	}

	alias := encodeTaskAliasID(c.NextID)
	c.NextID++
	c.Entries = append(c.Entries, taskAliasCacheEntry{
		UUID:           uuid,
		Alias:          alias,
		CreatedAt:      now,
		LastAccessedAt: now,
	})
	c.sortEntries()
	// Rebuild maps after append because sortEntries may reorder the slice and
	// the previous pointer stored in byUUID for other entries may now be stale.
	c.rebuildMaps()
	return alias, true
}

// lookupUUIDByAlias returns the UUID for the given alias using an O(1) map
// lookup. Returns (uuid, found, changed) where changed is true when
// LastAccessedAt was updated.
func (c *taskAliasCache) lookupUUIDByAlias(alias string, now time.Time) (string, bool, bool) {
	entry, ok := c.byAlias[alias]
	if !ok {
		return "", false, false
	}
	changed := !entry.LastAccessedAt.Equal(now)
	entry.LastAccessedAt = now
	return entry.UUID, true, changed
}

// lookupAliasByUUID returns the alias for the given UUID using an O(1) map
// lookup. Returns (alias, found, changed) where changed is true when
// LastAccessedAt was updated.
func (c *taskAliasCache) lookupAliasByUUID(uuid string, now time.Time) (string, bool, bool) {
	entry, ok := c.byUUID[uuid]
	if !ok {
		return "", false, false
	}
	changed := !entry.LastAccessedAt.Equal(now)
	entry.LastAccessedAt = now
	return entry.Alias, true, changed
}

func (c *taskAliasCache) save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create task alias cache dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal task alias cache: %w", err)
	}

	// Use a unique temp filename per invocation so two concurrent saves cannot
	// clobber each other's tempfile and produce a "rename: no such file or
	// directory" error on the loser.
	tempFile, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create task alias cache temp: %w", err)
	}
	tempPath := tempFile.Name()
	if err := writeAndCloseTaskAliasTemp(tempFile, tempPath, data); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace task alias cache: %w", err)
	}
	return nil
}

func writeAndCloseTaskAliasTemp(f *os.File, path string, data []byte) error {
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write task alias cache: %w", err)
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return fmt.Errorf("chmod task alias cache: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close task alias cache temp %s: %w", path, err)
	}
	return nil
}

func (c *taskAliasCache) sortEntries() {
	slices.SortFunc(c.Entries, func(a, b taskAliasCacheEntry) int {
		switch {
		case a.UUID < b.UUID:
			return -1
		case a.UUID > b.UUID:
			return 1
		default:
			return 0
		}
	})
}

func (e taskAliasCacheEntry) lastTouchedAt() time.Time {
	if !e.LastAccessedAt.IsZero() {
		return e.LastAccessedAt
	}
	return e.CreatedAt
}

// reverseString returns s with its bytes in reverse order. Alias strings only
// ever contain ASCII characters so byte-level reversal is correct.
func reverseString(s string) string {
	b := []byte(s)
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

// encodeTaskAliasID converts a monotonically-increasing counter to a short
// alphanumeric string and then reverses it. The reversal ensures that the
// first character of the alias varies as quickly as possible, which makes
// shell tab-completion more effective (e.g. "1", "2", ... "z", "00" becomes
// "1", "2", ..., "z", "00"; then "10", "20", ... instead of "00", "10", ...).
func encodeTaskAliasID(id uint64) string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

	width := 1
	blockSize := uint64(len(alphabet))
	remaining := id
	for remaining >= blockSize {
		remaining -= blockSize
		width++
		blockSize *= uint64(len(alphabet))
	}

	buf := make([]byte, width)
	for i := width - 1; i >= 0; i-- {
		buf[i] = alphabet[remaining%uint64(len(alphabet))]
		remaining /= uint64(len(alphabet))
	}
	// Reverse so that the least-significant digit comes first, keeping the
	// leading character diverse across consecutive IDs.
	return reverseString(string(buf))
}

// decodeTaskAliasID is the inverse of encodeTaskAliasID. It reverses the alias
// string to restore the canonical (most-significant-digit-first) form before
// decoding.
func decodeTaskAliasID(alias string) (uint64, bool) {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

	if alias == "" {
		return 0, false
	}

	// Reverse the alias back to the canonical form before decoding.
	canonical := reverseString(alias)
	width := len(canonical)
	var id uint64
	blockSize := uint64(len(alphabet))
	for i := 1; i < width; i++ {
		id += blockSize
		blockSize *= uint64(len(alphabet))
	}

	var value uint64
	for _, r := range canonical {
		index := int64(-1)
		for i, candidate := range alphabet {
			if r == candidate {
				index = int64(i)
				break
			}
		}
		if index < 0 {
			return 0, false
		}
		value = value*uint64(len(alphabet)) + uint64(index)
	}
	return id + value, true
}
