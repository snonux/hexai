package askcli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"codeberg.org/snonux/hexai/internal/stats"
)

const taskAliasCacheTTL = 120 * 24 * time.Hour

var (
	nowTaskAliasCache  = time.Now
	taskAliasCacheRoot = stats.CacheDir
)

type taskAliasCache struct {
	NextID  uint64                `json:"next_id"`
	Entries []taskAliasCacheEntry `json:"entries"`
}

type taskAliasCacheEntry struct {
	UUID           string    `json:"uuid"`
	Alias          string    `json:"alias"`
	CreatedAt      time.Time `json:"created_at"`
	LastAccessedAt time.Time `json:"last_accessed_at"`
}

func ensureTaskAliases(tasks []TaskExport) (map[string]string, error) {
	cache, path, err := loadTaskAliasCache()
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

func loadTaskAliasCache() (taskAliasCache, string, error) {
	path, err := taskAliasCachePath()
	if err != nil {
		return taskAliasCache{}, "", err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return taskAliasCache{}, path, nil
	}
	if err != nil {
		return taskAliasCache{}, "", fmt.Errorf("read task alias cache: %w", err)
	}

	var cache taskAliasCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return taskAliasCache{}, "", fmt.Errorf("parse task alias cache: %w", err)
	}
	if err := cache.validate(); err != nil {
		return taskAliasCache{}, "", fmt.Errorf("validate task alias cache: %w", err)
	}
	return cache, path, nil
}

func taskAliasCachePath() (string, error) {
	dir, err := taskAliasCacheRoot()
	if err != nil {
		return "", fmt.Errorf("resolve cache dir: %w", err)
	}
	return filepath.Join(dir, "ask", "task-aliases-v1.json"), nil
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
	}
	return changed
}

func (c *taskAliasCache) ensureAlias(uuid string, now time.Time) (string, bool) {
	for i := range c.Entries {
		if c.Entries[i].UUID != uuid {
			continue
		}
		if c.Entries[i].LastAccessedAt.Equal(now) {
			return c.Entries[i].Alias, false
		}
		c.Entries[i].LastAccessedAt = now
		return c.Entries[i].Alias, true
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
	return alias, true
}

func (c taskAliasCache) save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create task alias cache dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal task alias cache: %w", err)
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return fmt.Errorf("write task alias cache: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace task alias cache: %w", err)
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
	return string(buf)
}

func decodeTaskAliasID(alias string) (uint64, bool) {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

	if alias == "" {
		return 0, false
	}

	width := len(alias)
	var id uint64
	blockSize := uint64(len(alphabet))
	for i := 1; i < width; i++ {
		id += blockSize
		blockSize *= uint64(len(alphabet))
	}

	var value uint64
	for _, r := range alias {
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
