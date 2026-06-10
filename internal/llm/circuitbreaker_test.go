package llm

import (
	"testing"
	"time"
)

func TestCircuitBreaker_NilIsNoOp(t *testing.T) {
	var cb *circuitBreaker
	if !cb.Allow() {
		t.Fatal("nil breaker must allow")
	}
	// These must not panic on a nil receiver.
	cb.recordFailure()
	cb.recordSuccess()
}

func TestCircuitBreaker_TripsAfterThreshold(t *testing.T) {
	cb := newCircuitBreaker(3, time.Minute)
	for i := 0; i < 2; i++ {
		cb.recordFailure()
		if !cb.Allow() {
			t.Fatalf("breaker should stay closed before threshold (i=%d)", i)
		}
	}
	cb.recordFailure() // third failure trips it
	if cb.Allow() {
		t.Fatal("breaker should be open after threshold failures")
	}
}

func TestCircuitBreaker_SuccessResets(t *testing.T) {
	cb := newCircuitBreaker(2, time.Minute)
	cb.recordFailure()
	cb.recordSuccess()
	cb.recordFailure() // only one failure since reset; should not trip
	if !cb.Allow() {
		t.Fatal("breaker should remain closed after success reset")
	}
}

func TestCircuitBreaker_HalfOpenAllowsSingleProbe(t *testing.T) {
	now := time.Unix(0, 0)
	cb := newCircuitBreaker(1, 10*time.Second)
	cb.now = func() time.Time { return now }

	cb.recordFailure() // trips (threshold 1)
	if cb.Allow() {
		t.Fatal("should be open immediately after trip")
	}

	// Advance past cooldown: first Allow transitions to half-open and permits one probe.
	now = now.Add(11 * time.Second)
	if !cb.Allow() {
		t.Fatal("expected half-open probe to be allowed")
	}
	// A second concurrent probe is rejected.
	if cb.Allow() {
		t.Fatal("second probe must be rejected while half-open")
	}
}

func TestCircuitBreaker_HalfOpenProbeSuccessCloses(t *testing.T) {
	now := time.Unix(0, 0)
	cb := newCircuitBreaker(1, time.Second)
	cb.now = func() time.Time { return now }

	cb.recordFailure()
	now = now.Add(2 * time.Second)
	if !cb.Allow() {
		t.Fatal("expected probe allowed")
	}
	cb.recordSuccess()
	if !cb.Allow() {
		t.Fatal("breaker should be closed after successful probe")
	}
}

func TestCircuitBreaker_HalfOpenProbeFailureReopens(t *testing.T) {
	now := time.Unix(0, 0)
	cb := newCircuitBreaker(1, time.Second)
	cb.now = func() time.Time { return now }

	cb.recordFailure()
	now = now.Add(2 * time.Second)
	if !cb.Allow() {
		t.Fatal("expected probe allowed")
	}
	cb.recordFailure() // probe failed: reopen
	if cb.Allow() {
		t.Fatal("breaker should reopen after failed probe")
	}
}

func TestCircuitBreaker_ZeroThresholdNeverTrips(t *testing.T) {
	cb := newCircuitBreaker(0, time.Minute)
	for i := 0; i < 100; i++ {
		cb.recordFailure()
	}
	if !cb.Allow() {
		t.Fatal("threshold<=0 should never trip the breaker")
	}
}
