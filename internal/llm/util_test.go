package llm

import "testing"

func TestNilStringErr(t *testing.T) {
	s, err := nilStringErr("boom")
	if s != "" || err == nil {
		t.Fatalf("expected empty string and error")
	}
}
