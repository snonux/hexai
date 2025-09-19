//go:build !windows

package stats

import (
	"errors"

	"golang.org/x/sys/unix"
)

func tryLockFile(fd uintptr) error {
	if err := unix.Flock(int(fd), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) {
			return errLockWouldBlock
		}
		return err
	}
	return nil
}

func unlockFile(fd uintptr) error {
	return unix.Flock(int(fd), unix.LOCK_UN)
}
