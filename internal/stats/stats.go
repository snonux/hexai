// Package stats provides a simple, process-safe, on-disk cache of Hexai LLM usage
// statistics shared across all binaries. It appends compact events (ts, provider,
// model, sent, recv) to a JSON file guarded by an advisory file lock, prunes
// entries older than the configured window (default 1h), and computes aggregated
// snapshots for display in logs and tmux status.
package stats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"codeberg.org/snonux/hexai/internal/filelock"
)

const (
	fileName      = "stats.json"
	lockFileName  = "stats.lock"
	fileVersion   = 1
	defaultWindow = time.Hour
)

var windowSeconds int64 = int64(defaultWindow.Seconds())

// nowFunc is the clock source for event timestamps and pruning cutoffs.
// Replaced in tests to control time without sleeping.
var nowFunc = time.Now

// SetWindow sets the sliding window used for pruning and aggregation.
func SetWindow(d time.Duration) {
	if d < time.Second {
		d = time.Second
	}
	if d > 24*time.Hour {
		d = 24 * time.Hour
	}
	atomic.StoreInt64(&windowSeconds, int64(d.Seconds()))
}

// Window returns the current sliding window.
func Window() time.Duration { return time.Duration(atomic.LoadInt64(&windowSeconds)) * time.Second }

// Event represents a single request/response with sizes.
type Event struct {
	TS       time.Time `json:"ts"`
	Provider string    `json:"provider"`
	Model    string    `json:"model"`
	Sent     int64     `json:"sent"`
	Recv     int64     `json:"recv"`
}

// File is the on-disk JSON structure.
type File struct {
	Version       int       `json:"version"`
	UpdatedAt     time.Time `json:"updated_at"`
	WindowSeconds int       `json:"window_seconds"`
	Events        []Event   `json:"events"`
}

// Counters and Snapshot represent computed aggregates for the current window.
type Counters struct{ Reqs, Sent, Recv int64 }

type ProviderEntry struct {
	Totals Counters
	Models map[string]Counters
}

type Snapshot struct {
	Global    Counters
	Providers map[string]ProviderEntry
	RPM       float64
	Window    time.Duration
}

// ScopeReqs returns the request count for a specific provider+model pair.
// Returns 0 when the provider or model is not present in the snapshot.
func (s Snapshot) ScopeReqs(provider, model string) int64 {
	if pe, ok := s.Providers[provider]; ok {
		if mc, ok2 := pe.Models[model]; ok2 {
			return mc.Reqs
		}
	}
	return 0
}

// ScopeRPM returns the requests-per-minute for a specific provider+model
// pair, derived from ScopeReqs and the snapshot's sliding window.
func (s Snapshot) ScopeRPM(provider, model string) float64 {
	reqs := s.ScopeReqs(provider, model)
	if reqs == 0 {
		return 0
	}
	mins := s.Window.Minutes()
	if mins <= 0 {
		mins = 0.001
	}
	return float64(reqs) / mins
}

// Update appends one event and prunes old entries under lock.
func Update(ctx context.Context, provider, model string, sentBytes, recvBytes int) error {
	dir, err := CacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	unlock, err := lockStatsFile(ctx, dir)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()

	path := filepath.Join(dir, fileName)
	sf := readStatsFile(path)
	now := nowFunc()
	win := Window()
	sf.WindowSeconds = int(win.Seconds())
	sf.Events = append(sf.Events, Event{
		TS: now, Provider: provider, Model: model,
		Sent: int64(sentBytes), Recv: int64(recvBytes),
	})
	pruneOldEvents(&sf, now.Add(-win))
	sf.UpdatedAt = now
	return writeStatsFileAtomic(dir, path, &sf)
}

// lockStatsFile acquires an advisory file lock on the stats lock file within dir.
// Returns an unlock function on success.
func lockStatsFile(ctx context.Context, dir string) (func() error, error) {
	lockPath := filepath.Join(dir, lockFileName)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	unlock, err := filelock.AcquireExclusive(ctx, f)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	// Return a combined unlock+close function so the caller only needs one defer.
	return func() error {
		uErr := unlock()
		cErr := f.Close()
		if uErr != nil {
			return uErr
		}
		return cErr
	}, nil
}

// readStatsFile loads the on-disk stats file, returning a fresh File if it is
// missing, corrupt, or has an incompatible version.
func readStatsFile(path string) File {
	var sf File
	if b, err := os.ReadFile(path); err == nil {
		if uerr := json.Unmarshal(b, &sf); uerr != nil {
			fmt.Fprintf(os.Stderr, "stats: corrupt stats file %s: %v, starting fresh\n", path, uerr)
			return File{Version: fileVersion}
		}
	}
	if sf.Version != fileVersion {
		sf = File{Version: fileVersion}
	}
	return sf
}

// pruneOldEvents removes events older than cutoff in-place.
func pruneOldEvents(sf *File, cutoff time.Time) {
	i := 0
	for ; i < len(sf.Events); i++ {
		if !sf.Events[i].TS.Before(cutoff) {
			break
		}
	}
	if i > 0 {
		sf.Events = append([]Event(nil), sf.Events[i:]...)
	}
}

// writeStatsFileAtomic writes sf to path via a temp file + rename for crash safety.
func writeStatsFileAtomic(dir, path string, sf *File) error {
	tmp, err := os.CreateTemp(dir, fileName+".tmp.")
	if err != nil {
		return err
	}
	enc := json.NewEncoder(tmp)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(sf); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// TakeSnapshot reads the stats file and aggregates events within the stored
// window (falling back to the process-level Window() if the file has none).
// This is a pure read — it does not mutate global state.
func TakeSnapshot() (Snapshot, error) {
	dir, err := CacheDir()
	if err != nil {
		return Snapshot{}, err
	}
	path := filepath.Join(dir, fileName)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Snapshot{Providers: map[string]ProviderEntry{}, Window: Window()}, nil
		}
		return Snapshot{}, err
	}
	var sf File
	if err := json.Unmarshal(b, &sf); err != nil {
		return Snapshot{}, err
	}
	win := time.Duration(sf.WindowSeconds) * time.Second
	if win <= 0 {
		win = Window()
	}
	cutoff := nowFunc().Add(-win)
	snap := Snapshot{Providers: make(map[string]ProviderEntry), Window: win}
	for _, ev := range sf.Events {
		if ev.TS.Before(cutoff) {
			continue
		}
		snap.Global.Reqs++
		snap.Global.Sent += ev.Sent
		snap.Global.Recv += ev.Recv
		pe := snap.Providers[ev.Provider]
		if pe.Models == nil {
			pe.Models = make(map[string]Counters)
		}
		pe.Totals.Reqs++
		pe.Totals.Sent += ev.Sent
		pe.Totals.Recv += ev.Recv
		mc := pe.Models[ev.Model]
		mc.Reqs++
		mc.Sent += ev.Sent
		mc.Recv += ev.Recv
		pe.Models[ev.Model] = mc
		snap.Providers[ev.Provider] = pe
	}
	mins := win.Minutes()
	if mins <= 0 {
		mins = 0.001
	}
	snap.RPM = float64(snap.Global.Reqs) / mins
	return snap, nil
}

// CacheDir resolves the cache directory for stats.
func CacheDir() (string, error) {
	if x := os.Getenv("XDG_CACHE_HOME"); strings.TrimSpace(x) != "" {
		return filepath.Join(x, "hexai"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve home: %w", err)
	}
	return filepath.Join(home, ".local", "hexai", "cache"), nil
}

// DebugString returns a compact single-line view of a snapshot (useful for logs).
func (s Snapshot) DebugString() string {
	return "Σ reqs=" + strconv.FormatInt(s.Global.Reqs, 10) + " rpm=" + fmt.Sprintf("%.2f", s.RPM)
}
