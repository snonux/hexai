//go:build !linux

package askcli

import (
	"os"
	"syscall"
)

func lockHolderIsStale(pid int, expectedComm string) bool {
	if pid <= 0 {
		return false
	}
	_ = expectedComm
	proc, err := os.FindProcess(pid)
	if err != nil {
		return true
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return true
	}
	return false
}
