package stats

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codeberg.org/snonux/hexai/internal/filelock"
)

func TestUpdateAndSnapshot_Single(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	SetWindow(2 * time.Minute)
	if err := Update(context.Background(), "prov", "model", 10, 20); err != nil {
		t.Fatalf("update: %v", err)
	}
	snap, err := TakeSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap.Global.Reqs != 1 || snap.Global.Sent != 10 || snap.Global.Recv != 20 {
		t.Fatalf("unexpected snap: %+v", snap)
	}
	if snap.Providers["prov"].Totals.Reqs != 1 || snap.Providers["prov"].Models["model"].Reqs != 1 {
		t.Fatalf("missing provider/model aggregates: %+v", snap)
	}
}

func TestUpdate_PrunesOld_ByWindow(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	SetWindow(2 * time.Second)
	ctx := context.Background()

	// Inject a fake clock via an engine so we can advance time without sleeping
	// and without mutating package-level state.
	fakeNow := time.Now()
	eng := engine{now: func() time.Time { return fakeNow }}

	if err := eng.update(ctx, "p", "m", 1, 1); err != nil {
		t.Fatal(err)
	}
	// Advance fake time past the 2-second window so the first event is pruned.
	fakeNow = fakeNow.Add(3 * time.Second)
	if err := eng.update(ctx, "p", "m", 2, 2); err != nil {
		t.Fatal(err)
	}
	snap, err := eng.takeSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Global.Reqs != 1 || snap.Global.Sent != 2 || snap.Global.Recv != 2 {
		t.Fatalf("expected first event pruned, got %+v", snap)
	}
}

func TestConcurrentUpdates_LockSafety(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	SetWindow(1 * time.Minute)
	ctx := context.Background()
	var wg sync.WaitGroup
	n := 20
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := Update(ctx, "p", "m", i, i); err != nil {
				t.Errorf("update %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	snap, err := TakeSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Global.Reqs != int64(n) {
		t.Fatalf("reqs mismatch: %d", snap.Global.Reqs)
	}
}

func TestCacheDir_XDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	got, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "hexai")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// TestCacheDir_FallbackHome covers the branch where XDG_CACHE_HOME is unset,
// so CacheDir falls back to $HOME/.local/hexai/cache.
func TestCacheDir_FallbackHome(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	got, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".local", "hexai", "cache")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// TestCacheDir_WhitespaceXDG covers the branch where XDG_CACHE_HOME contains
// only whitespace, which strings.TrimSpace reduces to "" so the fallback is used.
func TestCacheDir_WhitespaceXDG(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "  \t\n ")
	got, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".local", "hexai", "cache")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// TestSetWindow_ClampLow covers the branch where d < 1s is clamped to 1s.
func TestSetWindow_ClampLow(t *testing.T) {
	SetWindow(100 * time.Millisecond)
	got := Window()
	if got != time.Second {
		t.Fatalf("expected 1s, got %v", got)
	}
}

// TestSetWindow_ClampHigh covers the branch where d > 24h is clamped to 24h.
func TestSetWindow_ClampHigh(t *testing.T) {
	SetWindow(48 * time.Hour)
	got := Window()
	if got != 24*time.Hour {
		t.Fatalf("expected 24h, got %v", got)
	}
	// Restore a reasonable default for other tests.
	SetWindow(time.Hour)
}

// TestStringsTrim_NoTrimNeeded covers the early-return branch where the input
// has no leading or trailing whitespace, so the original string is returned.
func TestStringsTrim_NoTrimNeeded(t *testing.T) {
	in := "hello"
	got := strings.TrimSpace(in)
	if got != "hello" {
		t.Fatalf("expected %q, got %q", "hello", got)
	}
}

// TestStringsTrim_AllWhitespace covers trimming a string that is entirely whitespace.
func TestStringsTrim_AllWhitespace(t *testing.T) {
	got := strings.TrimSpace("  \t\r\n  ")
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

// TestStringsTrim_LeadingAndTrailing covers trimming from both ends.
func TestStringsTrim_LeadingAndTrailing(t *testing.T) {
	got := strings.TrimSpace("  abc  ")
	if got != "abc" {
		t.Fatalf("expected %q, got %q", "abc", got)
	}
}

// TestStringsTrim_Empty covers the empty string edge case.
func TestStringsTrim_Empty(t *testing.T) {
	got := strings.TrimSpace("")
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

// TestUpdate_CorruptFile covers the branch where the existing stats file has
// invalid JSON or a wrong version, forcing a reset.
func TestUpdate_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	SetWindow(1 * time.Minute)

	// Write a corrupt stats file.
	statsDir := filepath.Join(dir, "hexai")
	if err := os.MkdirAll(statsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(statsDir, fileName), []byte("{invalid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Update should still succeed: the corrupt file is discarded.
	if err := Update(context.Background(), "p", "m", 5, 5); err != nil {
		t.Fatalf("update after corrupt file: %v", err)
	}
	snap, err := TakeSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Global.Reqs != 1 {
		t.Fatalf("expected 1 req, got %d", snap.Global.Reqs)
	}
}

// TestUpdate_WrongVersion covers the branch where the file version does not
// match fileVersion, causing a reset of the file structure.
func TestUpdate_WrongVersion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	SetWindow(1 * time.Minute)

	statsDir := filepath.Join(dir, "hexai")
	if err := os.MkdirAll(statsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a valid JSON file but with version=99 (wrong).
	wrongVer := File{Version: 99, Events: []Event{{TS: time.Now(), Provider: "old", Model: "old", Sent: 100, Recv: 100}}}
	b, _ := json.Marshal(wrongVer)
	if err := os.WriteFile(filepath.Join(statsDir, fileName), b, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Update(context.Background(), "p", "m", 1, 1); err != nil {
		t.Fatalf("update: %v", err)
	}
	snap, err := TakeSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	// The old event from version 99 should be discarded.
	if snap.Global.Reqs != 1 {
		t.Fatalf("expected 1 req after version reset, got %d", snap.Global.Reqs)
	}
}

// TestTakeSnapshot_NoFile covers the ErrNotExist branch in TakeSnapshot.
func TestTakeSnapshot_NoFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	SetWindow(5 * time.Minute)

	snap, err := TakeSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Global.Reqs != 0 {
		t.Fatalf("expected 0 reqs, got %d", snap.Global.Reqs)
	}
	if snap.Providers == nil {
		t.Fatal("expected non-nil Providers map")
	}
}

// TestTakeSnapshot_BadJSON covers the json.Unmarshal error branch in TakeSnapshot.
func TestTakeSnapshot_BadJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	statsDir := filepath.Join(dir, "hexai")
	if err := os.MkdirAll(statsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(statsDir, fileName), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := TakeSnapshot()
	if err == nil {
		t.Fatal("expected error for bad JSON, got nil")
	}
}

// TestTakeSnapshot_ZeroWindowSeconds covers the branch where the file has
// WindowSeconds <= 0, causing TakeSnapshot to use the process-level Window().
func TestTakeSnapshot_ZeroWindowSeconds(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	SetWindow(5 * time.Minute)

	statsDir := filepath.Join(dir, "hexai")
	if err := os.MkdirAll(statsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sf := File{
		Version:       fileVersion,
		WindowSeconds: 0, // triggers the win <= 0 branch
		Events:        []Event{{TS: time.Now(), Provider: "p", Model: "m", Sent: 1, Recv: 1}},
	}
	b, _ := json.Marshal(sf)
	if err := os.WriteFile(filepath.Join(statsDir, fileName), b, 0o644); err != nil {
		t.Fatal(err)
	}

	snap, err := TakeSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Window != 5*time.Minute {
		t.Fatalf("expected 5m window fallback, got %v", snap.Window)
	}
	if snap.Global.Reqs != 1 {
		t.Fatalf("expected 1 req, got %d", snap.Global.Reqs)
	}
}

// TestUpdate_CancelledContext covers the context cancellation branch in
// filelock.AcquireExclusive when the lock is already held.
func TestUpdate_CancelledContext(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	SetWindow(1 * time.Minute)

	statsDir := filepath.Join(dir, "hexai")
	if err := os.MkdirAll(statsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Hold the lock file to force filelock.AcquireExclusive to spin.
	lockPath := filepath.Join(statsDir, lockFileName)
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lf.Close() }()
	unlock, err := filelock.AcquireExclusive(context.Background(), lf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unlock() }()

	// Now try to Update with an already-cancelled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = Update(ctx, "p", "m", 1, 1)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

func TestSnapshot_ScopeReqs(t *testing.T) {
	snap := Snapshot{
		Providers: map[string]ProviderEntry{
			"openai": {Models: map[string]Counters{"gpt-5.0": {Reqs: 42}}},
		},
	}
	if got := snap.ScopeReqs("openai", "gpt-5.0"); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
	if got := snap.ScopeReqs("openai", "gpt-4.1"); got != 0 {
		t.Fatalf("expected 0 for missing model, got %d", got)
	}
	if got := snap.ScopeReqs("anthropic", "gpt-5.0"); got != 0 {
		t.Fatalf("expected 0 for missing provider, got %d", got)
	}
}

func TestSnapshot_ScopeRPM(t *testing.T) {
	snap := Snapshot{
		Providers: map[string]ProviderEntry{
			"openai": {Models: map[string]Counters{"gpt-5.0": {Reqs: 60}}},
		},
		Window: time.Hour,
	}
	rpm := snap.ScopeRPM("openai", "gpt-5.0")
	if rpm != 1.0 {
		t.Fatalf("expected 1.0 rpm, got %v", rpm)
	}
	// Missing model should return 0
	if rpm := snap.ScopeRPM("openai", "missing"); rpm != 0 {
		t.Fatalf("expected 0 rpm for missing, got %v", rpm)
	}
}

func TestSnapshot_ScopeRPM_ZeroWindow(t *testing.T) {
	snap := Snapshot{
		Providers: map[string]ProviderEntry{
			"openai": {Models: map[string]Counters{"gpt-5.0": {Reqs: 10}}},
		},
		Window: 0, // edge case
	}
	rpm := snap.ScopeRPM("openai", "gpt-5.0")
	if rpm <= 0 {
		t.Fatalf("expected positive rpm even with zero window, got %v", rpm)
	}
}
