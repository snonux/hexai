//go:build windows

package stats

import (
	"golang.org/x/sys/windows"
)

func tryLockFile(fd uintptr) error {
	var ol windows.Overlapped
	err := windows.LockFileEx(windows.Handle(fd), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
	if err == nil {
		return nil
	}
	if err == windows.ERROR_LOCK_VIOLATION {
		return errLockWouldBlock
	}
	return err
}

func unlockFile(fd uintptr) error {
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(fd), 0, 1, 0, &ol)
}
