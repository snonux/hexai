package askcli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type resolvedTaskSelector struct {
	Input     string
	UUID      string
	Alias     string
	UsedAlias bool
}

func (d *Dispatcher) resolveTaskSelector(ctx context.Context, selector string, stderr io.Writer) (resolvedTaskSelector, []TaskExport, int, error) {
	normalized, requiresLookup, err := normalizeTaskSelectorInput(selector)
	if err != nil {
		return resolvedTaskSelector{}, nil, 1, err
	}

	resolved, err := d.aliasCache.withDefaults().resolveTaskSelectorFromCache(normalized, requiresLookup)
	if err != nil {
		return resolvedTaskSelector{}, nil, 1, err
	}

	tasks, code, err := d.exportTasks(ctx, []string{"uuid:" + resolved.UUID, "export"}, stderr)
	if err != nil {
		if resolved.UsedAlias && strings.Contains(err.Error(), "task not found") {
			return resolvedTaskSelector{}, nil, 1, fmt.Errorf("alias %q did not resolve to a task in the current scope", selector)
		}
		return resolvedTaskSelector{}, nil, code, err
	}

	aliases, err := d.aliasCache.withDefaults().ensureTaskAliases(tasks)
	if err != nil {
		return resolvedTaskSelector{}, nil, 1, err
	}
	if alias, ok := aliases[resolved.UUID]; ok {
		resolved.Alias = alias
	}
	return resolved, tasks, 0, nil
}

func normalizeTaskSelectorInput(selector string) (string, bool, error) {
	normalized := NormalizeUUID(selector)
	if selector != normalized && IsNumericID(normalized) {
		return "", false, fmt.Errorf("%s", strings.TrimSpace(RejectNumericID()))
	}
	return normalized, selector == normalized, nil
}

func resolveTaskSelectorFromCache(selector string, allowAlias bool) (resolvedTaskSelector, error) {
	return defaultTaskAliasCacheDeps().resolveTaskSelectorFromCache(selector, allowAlias)
}

func (d taskAliasCacheDeps) resolveTaskSelectorFromCache(selector string, allowAlias bool) (resolvedTaskSelector, error) {
	d = d.withDefaults()
	resolved := resolvedTaskSelector{Input: selector, UUID: selector}
	if !allowAlias || !looksLikeTaskAlias(selector) {
		return resolved, nil
	}

	path, err := d.taskAliasCachePath()
	if err != nil {
		return resolvedTaskSelector{}, err
	}
	unlock, err := d.lock(filepath.Dir(path))
	if err != nil {
		return resolvedTaskSelector{}, err
	}
	defer func() { _ = unlock() }()

	cache, err := loadTaskAliasCacheAt(path)
	if err != nil {
		return resolvedTaskSelector{}, err
	}

	now := d.now().UTC()
	changed := cache.prune(now)
	uuidFromAlias, aliasFound, aliasChanged := cache.lookupUUIDByAlias(selector, now)
	aliasForUUID, uuidFound, uuidChanged := cache.lookupAliasByUUID(selector, now)
	changed = changed || aliasChanged || uuidChanged

	return finalizeResolvedTaskSelector(&cache, path, selector,
		uuidFromAlias, aliasForUUID, aliasFound, uuidFound, changed)
}

// finalizeResolvedTaskSelector applies the lookup results, persisting any
// LastAccessedAt updates first when changed is true.
func finalizeResolvedTaskSelector(
	cache *taskAliasCache,
	path, selector, uuidFromAlias, aliasForUUID string,
	aliasFound, uuidFound, changed bool,
) (resolvedTaskSelector, error) {
	saveIfChanged := func() error {
		if !changed {
			return nil
		}
		return cache.save(path)
	}
	switch {
	case aliasFound && uuidFound && uuidFromAlias != selector:
		if err := saveIfChanged(); err != nil {
			return resolvedTaskSelector{}, err
		}
		return resolvedTaskSelector{}, fmt.Errorf("task selector %q is ambiguous: it matches alias for %s and UUID %s; use uuid:%s to force UUID", selector, uuidFromAlias, selector, selector)
	case aliasFound:
		if err := saveIfChanged(); err != nil {
			return resolvedTaskSelector{}, err
		}
		return resolvedTaskSelector{Input: selector, UUID: uuidFromAlias, Alias: selector, UsedAlias: true}, nil
	case uuidFound:
		if err := saveIfChanged(); err != nil {
			return resolvedTaskSelector{}, err
		}
		return resolvedTaskSelector{Input: selector, UUID: selector, Alias: aliasForUUID}, nil
	}
	if IsNumericID(selector) {
		return resolvedTaskSelector{}, fmt.Errorf("%s", strings.TrimSpace(RejectNumericID()))
	}
	if err := saveIfChanged(); err != nil {
		return resolvedTaskSelector{}, err
	}
	return resolvedTaskSelector{}, fmt.Errorf("task selector %q did not match a known alias; use uuid:%s to force UUID", selector, selector)
}

func looksLikeTaskAlias(selector string) bool {
	_, ok := decodeTaskAliasID(selector)
	return ok && !strings.Contains(selector, "-")
}

func displayResolvedTaskID(resolved resolvedTaskSelector) string {
	return displayTaskAlias(resolved.UUID, map[string]string{resolved.UUID: resolved.Alias})
}

func displayTaskAlias(uuid string, aliases map[string]string) string {
	if alias := strings.TrimSpace(aliases[uuid]); alias != "" {
		return alias
	}
	return uuid
}

// NormalizeUUID strips any leading "uuid:" prefix so callers can accept
// both "uuid:<value>" and bare UUID strings interchangeably. The returned
// value is always a plain UUID ready to be prefixed again when building
// taskwarrior filter arguments.
func NormalizeUUID(s string) string {
	return strings.TrimPrefix(s, "uuid:")
}

// IsNumericID reports whether the string is entirely numeric.
func IsNumericID(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// RejectNumericID returns the error message emitted when numeric Taskwarrior IDs are passed.
func RejectNumericID() string {
	return "error: use a task alias ID or UUID, not a numeric Taskwarrior task ID\n"
}
