package askcli

import (
	"context"
	"fmt"
	"io"
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

	resolved, err := resolveTaskSelectorFromCache(normalized, requiresLookup)
	if err != nil {
		return resolvedTaskSelector{}, nil, 1, err
	}

	tasks, code, err := d.exportTasks(ctx, []string{"uuid:" + resolved.UUID, "export"}, stderr)
	if err != nil {
		if resolved.UsedAlias && strings.Contains(err.Error(), "task not found") {
			return resolvedTaskSelector{}, nil, 1, fmt.Errorf("alias %q is stale: task %s was not found", selector, resolved.UUID)
		}
		return resolvedTaskSelector{}, nil, code, err
	}

	aliases, err := ensureTaskAliases(tasks)
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
		return "", false, fmt.Errorf(strings.TrimSpace(RejectNumericID()))
	}
	return normalized, selector == normalized, nil
}

func resolveTaskSelectorFromCache(selector string, allowAlias bool) (resolvedTaskSelector, error) {
	resolved := resolvedTaskSelector{Input: selector, UUID: selector}
	if !allowAlias || !looksLikeTaskAlias(selector) {
		return resolved, nil
	}

	cache, path, err := loadTaskAliasCache()
	if err != nil {
		return resolvedTaskSelector{}, err
	}

	now := nowTaskAliasCache().UTC()
	changed := cache.prune(now)
	uuidFromAlias, aliasFound, aliasChanged := cache.lookupUUIDByAlias(selector, now)
	changed = changed || aliasChanged
	aliasForUUID, uuidFound, uuidChanged := cache.lookupAliasByUUID(selector, now)
	changed = changed || uuidChanged

	switch {
	case aliasFound && uuidFound && uuidFromAlias != selector:
		if changed {
			if err := cache.save(path); err != nil {
				return resolvedTaskSelector{}, err
			}
		}
		return resolvedTaskSelector{}, fmt.Errorf("task selector %q is ambiguous: it matches alias for %s and UUID %s; use uuid:%s to force UUID", selector, uuidFromAlias, selector, selector)
	case aliasFound:
		if changed {
			if err := cache.save(path); err != nil {
				return resolvedTaskSelector{}, err
			}
		}
		return resolvedTaskSelector{
			Input:     selector,
			UUID:      uuidFromAlias,
			Alias:     selector,
			UsedAlias: true,
		}, nil
	case uuidFound:
		if changed {
			if err := cache.save(path); err != nil {
				return resolvedTaskSelector{}, err
			}
		}
		return resolvedTaskSelector{
			Input: selector,
			UUID:  selector,
			Alias: aliasForUUID,
		}, nil
	default:
		if IsNumericID(selector) {
			return resolvedTaskSelector{}, fmt.Errorf(strings.TrimSpace(RejectNumericID()))
		}
		if changed {
			if err := cache.save(path); err != nil {
				return resolvedTaskSelector{}, err
			}
		}
		return resolvedTaskSelector{}, fmt.Errorf("task selector %q did not match a known alias; use uuid:%s to force UUID", selector, selector)
	}
}

func looksLikeTaskAlias(selector string) bool {
	_, ok := decodeTaskAliasID(selector)
	return ok && !strings.Contains(selector, "-")
}
