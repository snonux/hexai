package llm

import (
	"os"
	"testing"
)

// TestMain registers all built-in providers before any test runs, mirroring
// the explicit registration that happens in production binaries.
func TestMain(m *testing.M) {
	RegisterAllProviders()
	os.Exit(m.Run())
}

func f64p(v float64) *float64 { return &v }
