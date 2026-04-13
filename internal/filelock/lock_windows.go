//go:build windows

package filelock

import (
	"golang.org/x/sys/windows"
)

func tryLockExclusive(fd uintptr) error {
	var ol windows.Overlapped
	err := windows.LockFileEx(windows.Handle(fd), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
	if err == nil {
		return nil
	}
	if err == windows.ERROR_LOCK_VIOLATION {
		return ErrWouldBlock
	}
	return err
}

func unlockExclusive(fd uintptr) error {
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(fd), 0, 1, 0, &ol)
}
