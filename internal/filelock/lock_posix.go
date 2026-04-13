//go:build !windows

package filelock

import (
	"errors"

	"golang.org/x/sys/unix"
)

func tryLockExclusive(fd uintptr) error {
	if err := unix.Flock(int(fd), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return ErrWouldBlock
		}
		return err
	}
	return nil
}

func unlockExclusive(fd uintptr) error {
	return unix.Flock(int(fd), unix.LOCK_UN)
}
