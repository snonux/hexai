package filelock

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
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
