package lsp

import (
	"context"
	"testing"
	"time"
)

func TestWaitForDebounce_WaitsRoughlyDebounce(t *testing.T) {
	s := newTestServer()
	s.completionDebounce = 20 * time.Millisecond
	s.mu.Lock()
	s.lastInput = time.Now()
	s.mu.Unlock()
	start := time.Now()
	s.waitForDebounce(context.Background())
	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Fatalf("debounce did not wait long enough: %v", elapsed)
	}
}

func TestWaitForThrottle_WaitsRoughlyInterval(t *testing.T) {
	s := newTestServer()
	s.throttleInterval = 20 * time.Millisecond
	s.mu.Lock()
	s.lastLLMCall = time.Now()
	s.mu.Unlock()
	start := time.Now()
	if !s.waitForThrottle(context.Background()) {
		t.Fatalf("waitForThrottle returned false")
	}
	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Fatalf("throttle did not wait long enough: %v", elapsed)
	}
}
