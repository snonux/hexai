//go:build linux

package askcli

import (
	"os/exec"
	"testing"
)

func TestLockHolderIsStale_NonAskLiveProcess(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Skip("sleep not available:", err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	if !lockHolderIsStale(cmd.Process.Pid, "ask") {
		t.Fatal("expected sleep process to be stale when expecting ask")
	}
}
