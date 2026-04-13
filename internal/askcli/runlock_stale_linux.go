//go:build linux

package askcli

import (
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func lockHolderIsStale(pid int, expectedComm string) bool {
	if pid <= 0 {
		return false
	}
	if err := unix.Kill(pid, 0); err != nil {
		return true
	}
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return false
	}
	holder := strings.TrimSpace(string(data))
	if holder == "" {
		return false
	}
	want := expectedComm
	if len(want) > 15 {
		want = want[:15]
	}
	return holder != want
}
