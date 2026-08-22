package runtimeconfig

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/snonux/hexai/internal/appconfig"
)

// Change captures a single configuration delta.
type Change struct {
	Key string
	Old string
	New string
}

// Listener receives the previous and new application configuration when updates occur.
type Listener func(old appconfig.App, new appconfig.App)

// Store holds the active runtime configuration and notifies listeners on updates.
type Store struct {
	mu        sync.RWMutex
	cfg       appconfig.App
	listeners map[int]Listener
	nextID    int
}

// New creates a Store seeded with the provided configuration snapshot.
func New(cfg appconfig.App) *Store {
	return &Store{cfg: cfg, listeners: make(map[int]Listener)}
}

// Snapshot returns the current configuration snapshot. Callers must treat it as read-only.
func (s *Store) Snapshot() appconfig.App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Subscribe registers a listener that will be invoked on configuration changes.
// The returned function removes the listener.
func (s *Store) Subscribe(listener Listener) func() {
	if listener == nil {
		return func() {}
	}
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.listeners[id] = listener
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		delete(s.listeners, id)
		s.mu.Unlock()
	}
}

// Set replaces the current configuration with the provided snapshot and notifies listeners.
// It returns the list of detected changes between the previous and new configuration.
func (s *Store) Set(cfg appconfig.App) []Change {
	s.mu.Lock()
	old := s.cfg
	s.cfg = cfg
	listeners := make([]Listener, 0, len(s.listeners))
	for _, l := range s.listeners {
		listeners = append(listeners, l)
	}
	s.mu.Unlock()

	changes := Diff(old, cfg)
	for _, l := range listeners {
		l(old, cfg)
	}
	return changes
}

// Reload re-reads configuration using the supplied options and applies it when
// valid. ctx is forwarded to the config loader so a cancelled caller (e.g. a
// shutting-down server) aborts the blocking file reads.
func (s *Store) Reload(ctx context.Context, logger *log.Logger, opts appconfig.LoadOptions) ([]Change, error) {
	cfg := appconfig.LoadWithOptions(ctx, logger, opts)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	changes := s.Set(cfg)
	if logger != nil {
		logger.Print(FormatSummary("Reloaded config", changes))
	}
	return changes, nil
}

// Diff computes a stable, sorted list of key/value changes between two configuration snapshots.
func Diff(oldCfg, newCfg appconfig.App) []Change {
	before := flattenAppConfig(oldCfg)
	after := flattenAppConfig(newCfg)
	keys := make(map[string]struct{}, len(before)+len(after))
	for k := range before {
		keys[k] = struct{}{}
	}
	for k := range after {
		keys[k] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)
	changes := make([]Change, 0, len(ordered))
	for _, k := range ordered {
		if before[k] == after[k] {
			continue
		}
		changes = append(changes, Change{Key: k, Old: before[k], New: after[k]})
	}
	return changes
}

// flattenAppConfig converts an App config into a flat key/value map for diffing.
// It recurses into embedded structs (CoreConfig, ProviderConfig, etc.) to reach
// all leaf fields. Keys are derived from json tags, with fallbacks for fields
// that use json:"-" (e.g. surface configs, stats).
func flattenAppConfig(cfg appconfig.App) map[string]string {
	result := make(map[string]string)
	flattenStructFields(reflect.ValueOf(cfg), result)
	return result
}

// flattenStructFields iterates over struct fields, recursing into anonymous
// (embedded) structs and extracting key/value pairs from leaf fields.
func flattenStructFields(val reflect.Value, result map[string]string) {
	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		// Recurse into embedded (anonymous) structs to flatten their fields.
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			flattenStructFields(val.Field(i), result)
			continue
		}
		key := fieldKey(field)
		if key == "" {
			continue
		}
		result[key] = stringifyValue(val.Field(i))
	}
}

// fieldKey derives the flattened map key for a struct field from its json tag,
// with manual fallbacks for fields tagged json:"-" that still need tracking.
func fieldKey(field reflect.StructField) string {
	key := strings.TrimSpace(field.Tag.Get("json"))
	if key == "" || key == "-" {
		// Manual fallbacks for fields hidden from JSON but needed in diffs.
		switch field.Name {
		case "StatsWindowMinutes":
			return "stats_window_minutes"
		case "CompletionConfigs":
			return "completion_configs"
		case "CodeActionConfigs":
			return "code_action_configs"
		case "ChatConfigs":
			return "chat_configs"
		case "CLIConfigs":
			return "cli_configs"
		default:
			return ""
		}
	}
	if idx := strings.Index(key, ","); idx >= 0 {
		key = key[:idx]
	}
	return key
}

func stringifyValue(v reflect.Value) string {
	if !v.IsValid() {
		return ""
	}
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Slice:
		if v.IsNil() {
			return ""
		}
		if v.Type().Elem().Kind() == reflect.String {
			parts := make([]string, v.Len())
			for i := range parts {
				parts[i] = v.Index(i).String()
			}
			return strings.Join(parts, ",")
		}
		if v.Type().Elem() == reflect.TypeOf(appconfig.SurfaceConfig{}) {
			parts := make([]string, 0, v.Len())
			for i := 0; i < v.Len(); i++ {
				entry := v.Index(i).Interface().(appconfig.SurfaceConfig)
				segment := strings.TrimSpace(entry.Provider)
				if segment != "" {
					segment += ":"
				}
				segment += strings.TrimSpace(entry.Model)
				if entry.Temperature != nil {
					segment += fmt.Sprintf("@%.3f", *entry.Temperature)
				}
				parts = append(parts, segment)
			}
			return strings.Join(parts, "|")
		}
		return fmt.Sprint(v.Interface())
	case reflect.Ptr:
		if v.IsNil() {
			return "(unset)"
		}
		return stringifyValue(v.Elem())
	default:
		return fmt.Sprint(v.Interface())
	}
}

// FormatSummary creates a human-readable summary for configuration changes.
func FormatSummary(prefix string, changes []Change) string {
	if len(changes) == 0 {
		return fmt.Sprintf("%s (no changes detected).", prefix)
	}
	lines := make([]string, 0, len(changes)+1)
	lines = append(lines, fmt.Sprintf("%s (%d changes):", prefix, len(changes)))
	for _, ch := range changes {
		lines = append(lines, fmt.Sprintf("- %s: %s → %s", ch.Key, ch.Old, ch.New))
	}
	return strings.Join(lines, "\n")
}
