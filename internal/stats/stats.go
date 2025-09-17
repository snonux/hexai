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
	"sync/atomic"
	"syscall"
	"time"
)

const (
	fileName      = "stats.json"
	lockFileName  = "stats.lock"
	fileVersion   = 1
	defaultWindow = time.Hour
)

var windowSeconds int64 = int64(defaultWindow.Seconds())

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

// Update appends one event and prunes old entries under lock.
func Update(ctx context.Context, provider, model string, sentBytes, recvBytes int) error {
	dir, err := CacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	lockPath := filepath.Join(dir, lockFileName)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	// Acquire exclusive flock; best-effort ctx support via polling
	for {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			break
		}
		// Wait a bit or exit if context canceled
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
	// Read existing file (if any)
	path := filepath.Join(dir, fileName)
	var sf File
	if b, rerr := os.ReadFile(path); rerr == nil {
		_ = json.Unmarshal(b, &sf)
	}
	if sf.Version != fileVersion {
		sf = File{Version: fileVersion}
	}
	now := time.Now()
	win := Window()
	sf.WindowSeconds = int(win.Seconds())
	// Append event
	sf.Events = append(sf.Events, Event{TS: now, Provider: provider, Model: model, Sent: int64(sentBytes), Recv: int64(recvBytes)})
	// Prune old
	cutoff := now.Add(-win)
	if len(sf.Events) > 0 {
		// Find first >= cutoff
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
	sf.UpdatedAt = now
	// Write atomically
	tmp, err := os.CreateTemp(dir, fileName+".tmp.")
	if err != nil {
		return err
	}
	enc := json.NewEncoder(tmp)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(&sf); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// Snapshot reads and aggregates events within the configured window.
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
	} else {
		SetWindow(win) // align process with file window if changed elsewhere
	}
	cutoff := time.Now().Add(-win)
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
	if x := os.Getenv("XDG_CACHE_HOME"); stringsTrim(x) != "" {
		return filepath.Join(x, "hexai"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve home: %w", err)
	}
	return filepath.Join(home, ".cache", "hexai"), nil
}

// stringsTrim is a tiny helper to avoid importing strings everywhere here.
func stringsTrim(s string) string {
	i := 0
	j := len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n' || s[j-1] == '\r') {
		j--
	}
	if i == 0 && j == len(s) {
		return s
	}
	return s[i:j]
}

// DebugString returns a compact single-line view of a snapshot (useful for logs).
func (s Snapshot) DebugString() string {
	return "Σ reqs=" + strconv.FormatInt(s.Global.Reqs, 10) + " rpm=" + fmt.Sprintf("%.2f", s.RPM)
}
