// Package filelock provides advisory exclusive locks on open files.
package filelock

import (
	"context"
	"errors"
	"os"
	"time"
)

// ErrWouldBlock indicates a non-blocking lock attempt could not acquire the lock.
var ErrWouldBlock = errors.New("filelock: would block")

// TryExclusive attempts a non-blocking exclusive advisory lock on f.
func TryExclusive(f *os.File) error {
	return tryLockExclusive(f.Fd())
}

// UnlockExclusive releases the advisory lock held on f.
func UnlockExclusive(f *os.File) error {
	return unlockExclusive(f.Fd())
}

// retryInterval is the backoff between successive non-blocking lock attempts.
const retryInterval = 5 * time.Millisecond

// AcquireExclusive spins with TryExclusive until the lock is acquired, ctx is done, or a non-would-block error occurs.
func AcquireExclusive(ctx context.Context, f *os.File) (func() error, error) {
	fd := f.Fd()
	for {
		err := tryLockExclusive(fd)
		if err == nil {
			return func() error { return unlockExclusive(fd) }, nil
		}
		if errors.Is(err, ErrWouldBlock) {
			// Use a fresh timer per iteration via time.After instead of reusing and
			// Reset()-ing a single timer. Resetting a timer that may still be pending is
			// the documented hazard in the Go timer API: a stale value can already be
			// queued on the channel, causing a spurious early wake-up. A new timer each
			// loop guarantees a clean, full retryInterval delay (or ctx cancellation).
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryInterval):
			}
			continue
		}
		return nil, err
	}
}
