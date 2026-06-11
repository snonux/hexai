package policy

import (
	"testing"
	"time"
)

// TestRequestTimeouts verifies the documented default and research timeouts and
// that their whole-second mirrors stay in sync with the duration constants.
func TestRequestTimeouts(t *testing.T) {
	if DefaultRequestTimeout != 30*time.Second {
		t.Fatalf("DefaultRequestTimeout = %v, want 30s", DefaultRequestTimeout)
	}
	if ResearchRequestTimeout != 120*time.Second {
		t.Fatalf("ResearchRequestTimeout = %v, want 120s", ResearchRequestTimeout)
	}
	if DefaultRequestTimeoutSeconds != 30 {
		t.Fatalf("DefaultRequestTimeoutSeconds = %d, want 30", DefaultRequestTimeoutSeconds)
	}
	if ResearchRequestTimeoutSeconds != 120 {
		t.Fatalf("ResearchRequestTimeoutSeconds = %d, want 120", ResearchRequestTimeoutSeconds)
	}
	// The integer mirrors must equal the duration constants converted to seconds.
	if got := int(DefaultRequestTimeout / time.Second); got != DefaultRequestTimeoutSeconds {
		t.Fatalf("DefaultRequestTimeoutSeconds out of sync: %d vs %d", DefaultRequestTimeoutSeconds, got)
	}
	if got := int(ResearchRequestTimeout / time.Second); got != ResearchRequestTimeoutSeconds {
		t.Fatalf("ResearchRequestTimeoutSeconds out of sync: %d vs %d", ResearchRequestTimeoutSeconds, got)
	}
	// Research must allow a longer window than a normal chat completion.
	if ResearchRequestTimeout <= DefaultRequestTimeout {
		t.Fatalf("ResearchRequestTimeout (%v) must exceed DefaultRequestTimeout (%v)", ResearchRequestTimeout, DefaultRequestTimeout)
	}
}

// TestRetryPolicyConstants verifies the retry tuning values stay within sane,
// documented bounds.
func TestRetryPolicyConstants(t *testing.T) {
	if RetryMaxAttempts != 3 {
		t.Fatalf("RetryMaxAttempts = %d, want 3", RetryMaxAttempts)
	}
	if RetryBaseDelay != 200*time.Millisecond {
		t.Fatalf("RetryBaseDelay = %v, want 200ms", RetryBaseDelay)
	}
	if RetryMaxDelay != 2*time.Second {
		t.Fatalf("RetryMaxDelay = %v, want 2s", RetryMaxDelay)
	}
	if RetryBaseDelay > RetryMaxDelay {
		t.Fatalf("RetryBaseDelay (%v) must not exceed RetryMaxDelay (%v)", RetryBaseDelay, RetryMaxDelay)
	}
	if RetryJitterFraction <= 0 || RetryJitterFraction >= 1 {
		t.Fatalf("RetryJitterFraction = %v, want in (0,1)", RetryJitterFraction)
	}
}

// TestCircuitBreakerConstants verifies the circuit-breaker tuning values.
func TestCircuitBreakerConstants(t *testing.T) {
	if CircuitFailureThreshold != 5 {
		t.Fatalf("CircuitFailureThreshold = %d, want 5", CircuitFailureThreshold)
	}
	if CircuitCooldown != 30*time.Second {
		t.Fatalf("CircuitCooldown = %v, want 30s", CircuitCooldown)
	}
	if CircuitFailureThreshold <= 0 {
		t.Fatalf("CircuitFailureThreshold must be positive to trip the breaker")
	}
}
