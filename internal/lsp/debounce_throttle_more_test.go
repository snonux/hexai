package lsp

import (
	"context"
	"testing"
	"time"
)

func TestWaitForDebounce_WaitsRoughlyDebounce(t *testing.T) {
	s := newTestServer()
	cfg := s.cfg
	cfg.CompletionDebounceMs = 20
	s.cfg = cfg
	s.chatSvc().markActivity()
	start := time.Now()
	s.completionSvc().waitForDebounce(context.Background())
	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Fatalf("debounce did not wait long enough: %v", elapsed)
	}
}

func TestWaitForThrottle_WaitsRoughlyInterval(t *testing.T) {
	s := newTestServer()
	cfg := s.cfg
	cfg.CompletionThrottleMs = 20
	s.cfg = cfg
	cs := s.completionSvc()
	cs.stateMu.Lock()
	cs.lastLLMCall = time.Now()
	cs.stateMu.Unlock()
	start := time.Now()
	if !s.waitForThrottle(context.Background()) {
		t.Fatalf("waitForThrottle returned false")
	}
	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Fatalf("throttle did not wait long enough: %v", elapsed)
	}
}
