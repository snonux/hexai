package askcli

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/snonux/hexai/internal/filelock"
)

type lockResult struct {
	unlock func() error
	err    error
}

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

func TestAcquireAskRepoLock_StaleMetadataDoesNotRotateContendedLockFile(t *testing.T) {
	tmp := t.TempDir()
	holder, lockPath, origInfo := prepareContendedStaleLock(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resultCh := acquireLockAsync(ctx, tmp)

	select {
	case result := <-resultCh:
		if result.unlock != nil {
			_ = result.unlock()
		}
		t.Fatalf("lock acquired while holder still held lock: %v", result.err)
	case <-time.After(40 * time.Millisecond):
	}

	curInfo, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("stat contended lock: %v", err)
	}
	if !os.SameFile(origInfo, curInfo) {
		t.Fatal("contended lock file was replaced while locked")
	}

	releaseContendedLock(t, holder)

	result := <-resultCh
	if result.err != nil {
		t.Fatalf("contender lock: %v", result.err)
	}
	if result.unlock == nil {
		t.Fatal("contender returned nil unlock")
	}
	if err := result.unlock(); err != nil {
		t.Fatalf("contender unlock: %v", err)
	}
}

func prepareContendedStaleLock(t *testing.T, gitRoot string) (*os.File, string, os.FileInfo) {
	t.Helper()
	lockDir := filepath.Join(gitRoot, ".git")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(lockDir, askRepoLockFile)
	holder, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := filelock.TryExclusive(holder); err != nil {
		t.Fatalf("holder lock: %v", err)
	}
	if err := writeLockMetadata(holder, 999999, "ask"); err != nil {
		t.Fatalf("write stale metadata: %v", err)
	}
	origInfo, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("stat original lock: %v", err)
	}
	return holder, lockPath, origInfo
}

func acquireLockAsync(ctx context.Context, gitRoot string) <-chan lockResult {
	resultCh := make(chan lockResult, 1)
	go func() {
		unlock, err := acquireAskRepoLock(ctx, gitRoot)
		resultCh <- lockResult{unlock: unlock, err: err}
	}()
	return resultCh
}

func releaseContendedLock(t *testing.T, holder *os.File) {
	t.Helper()
	if err := filelock.UnlockExclusive(holder); err != nil {
		t.Fatalf("release holder lock: %v", err)
	}
	if err := holder.Close(); err != nil {
		t.Fatalf("close holder file: %v", err)
	}
}
