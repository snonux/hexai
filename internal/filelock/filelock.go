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

// AcquireExclusive spins with TryExclusive until the lock is acquired, ctx is done, or a non-would-block error occurs.
func AcquireExclusive(ctx context.Context, f *os.File) (func() error, error) {
	fd := f.Fd()
	retryTimer := time.NewTimer(5 * time.Millisecond)
	defer retryTimer.Stop()
	for {
		err := tryLockExclusive(fd)
		if err == nil {
			return func() error { return unlockExclusive(fd) }, nil
		}
		if errors.Is(err, ErrWouldBlock) {
			retryTimer.Reset(5 * time.Millisecond)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-retryTimer.C:
			}
			continue
		}
		return nil, err
	}
}
