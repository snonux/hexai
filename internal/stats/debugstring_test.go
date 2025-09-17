package stats

import "testing"

func TestSnapshotDebugString(t *testing.T) {
	s := Snapshot{}
	s.Global.Reqs = 42
	s.RPM = 3.14
	got := s.DebugString()
	if got == "" || !contains(got, "Σ reqs=42") || !contains(got, "rpm=") {
		t.Fatalf("unexpected debug string: %q", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
