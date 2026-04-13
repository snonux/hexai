package askcli

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquireAskRepoLock_SerializesConcurrentHolders(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	var maxHeld int32
	var cur int32
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock, err := acquireAskRepoLock(context.Background(), tmp)
			if err != nil {
				t.Errorf("lock: %v", err)
				return
			}
			defer func() { _ = unlock() }()
			n := atomic.AddInt32(&cur, 1)
			for {
				old := atomic.LoadInt32(&maxHeld)
				if n <= old || atomic.CompareAndSwapInt32(&maxHeld, old, n) {
					break
				}
			}
			time.Sleep(25 * time.Millisecond)
			atomic.AddInt32(&cur, -1)
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&maxHeld); got != 1 {
		t.Fatalf("max concurrent lock holders = %d, want 1", got)
	}
}
