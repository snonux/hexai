package filelock

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTryExclusive_SecondDescriptorWouldBlock(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "lock")
	f1, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f1.Close() })
	f2, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f2.Close() })
	if err := TryExclusive(f1); err != nil {
		t.Fatalf("first TryExclusive: %v", err)
	}
	err = TryExclusive(f2)
	if !errors.Is(err, ErrWouldBlock) {
		t.Fatalf("second TryExclusive = %v, want ErrWouldBlock", err)
	}
	if err := UnlockExclusive(f1); err != nil {
		t.Fatal(err)
	}
	if err := TryExclusive(f2); err != nil {
		t.Fatalf("after unlock: %v", err)
	}
	if err := UnlockExclusive(f2); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireExclusive_ContextCancelledWhileBlocked(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "lock")
	fHeld, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fHeld.Close() })
	if err := TryExclusive(fHeld); err != nil {
		t.Fatal(err)
	}
	fWait, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fWait.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = AcquireExclusive(ctx, fWait)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("AcquireExclusive = %v, want context.Canceled", err)
	}
	if err := UnlockExclusive(fHeld); err != nil {
		t.Fatal(err)
	}
}

// Exercises the retry-then-success path inside AcquireExclusive: the lock is
// initially held, the helper releases it after a short wait, and the waiting
// caller must observe a Flock retry succeed and return a working unlock fn.
func TestAcquireExclusive_RetriesAndSucceeds(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "lock")

	fHeld, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fHeld.Close() })
	if err := TryExclusive(fHeld); err != nil {
		t.Fatal(err)
	}

	fWait, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fWait.Close() })

	released := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = UnlockExclusive(fHeld)
		close(released)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	unlock, err := AcquireExclusive(ctx, fWait)
	if err != nil {
		t.Fatalf("AcquireExclusive = %v, want nil after retry", err)
	}
	<-released
	if err := unlock(); err != nil {
		t.Fatalf("unlock returned: %v", err)
	}
}

// Drives the non-EWOULDBLOCK error path: closing the file before calling
// AcquireExclusive yields EBADF, which must propagate up unchanged instead of
// being mapped to ErrWouldBlock or causing an infinite retry.
func TestAcquireExclusive_NonBlockingError_ReturnsImmediately(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "lock")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err = AcquireExclusive(ctx, f)
	if err == nil {
		t.Fatalf("AcquireExclusive on closed fd: got nil, want non-nil error")
	}
	if errors.Is(err, ErrWouldBlock) {
		t.Fatalf("AcquireExclusive on closed fd: got ErrWouldBlock, want underlying syscall error")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("AcquireExclusive on closed fd: got DeadlineExceeded, want immediate syscall error")
	}
}
