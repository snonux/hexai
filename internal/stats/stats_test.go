package stats

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
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
	if err := Update(ctx, "p", "m", 1, 1); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2200 * time.Millisecond)
	if err := Update(ctx, "p", "m", 2, 2); err != nil {
		t.Fatal(err)
	}
	snap, err := TakeSnapshot()
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
