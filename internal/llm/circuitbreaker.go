package llm

import (
	"sync"
	"time"
)

// circuitState models the three states of the classic circuit-breaker pattern.
//
//   - closed:   normal operation, requests flow through.
//   - open:     too many consecutive failures occurred; requests are rejected
//     immediately for a cooldown window to give the upstream time to recover.
//   - halfOpen: the cooldown elapsed; a single trial request is allowed through
//     to probe whether the upstream has recovered.
type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

// circuitBreaker is a small, dependency-free circuit breaker used to protect
// the LLM HTTP call path. It trips open after a configurable number of
// consecutive failures and stays open for a cooldown window, after which it
// allows a single trial ("half-open") request. A success in any state resets
// the breaker to closed.
//
// The breaker only counts failures that the retry layer deemed transient
// (network errors, 5xx, 429). Client errors (4xx) are not failures from the
// breaker's perspective: they indicate a bad request, not an unhealthy
// upstream, so they must not trip the circuit.
//
// A nil *circuitBreaker is a valid no-op breaker: Allow always returns true and
// the record* methods do nothing. This lets callers disable the breaker simply
// by passing nil.
type circuitBreaker struct {
	mu sync.Mutex

	// threshold is the number of consecutive failures that trips the breaker.
	threshold int
	// cooldown is how long the breaker stays open before allowing a trial.
	cooldown time.Duration
	// now is injectable for deterministic tests; defaults to time.Now.
	now func() time.Time

	state        circuitState
	failures     int
	openedAt     time.Time
	probeRunning bool
}

// newCircuitBreaker returns a breaker that trips after `threshold` consecutive
// transient failures and stays open for `cooldown`. A threshold <= 0 disables
// tripping (the breaker stays closed forever), which callers can use to turn
// the breaker off without special-casing nil.
func newCircuitBreaker(threshold int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{
		threshold: threshold,
		cooldown:  cooldown,
		now:       time.Now,
		state:     circuitClosed,
	}
}

// Allow reports whether a request may proceed under the current breaker state.
// When the breaker is open and the cooldown has elapsed it transitions to
// half-open and permits exactly one trial request; concurrent callers during
// half-open are rejected until the trial resolves via recordSuccess/recordFailure.
func (cb *circuitBreaker) Allow() bool {
	if cb == nil {
		return true
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitClosed:
		return true
	case circuitOpen:
		if cb.now().Sub(cb.openedAt) < cb.cooldown {
			return false
		}
		// Cooldown elapsed: move to half-open and allow a single probe.
		cb.state = circuitHalfOpen
		cb.probeRunning = true
		return true
	case circuitHalfOpen:
		// Only one probe at a time while half-open.
		if cb.probeRunning {
			return false
		}
		cb.probeRunning = true
		return true
	default:
		return true
	}
}

// recordSuccess resets the breaker to its healthy (closed) state. A success
// from a half-open probe means the upstream recovered.
func (cb *circuitBreaker) recordSuccess() {
	if cb == nil {
		return
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = circuitClosed
	cb.failures = 0
	cb.probeRunning = false
}

// recordFailure registers a transient failure. While half-open it re-opens the
// breaker immediately (the probe failed). While closed it trips the breaker
// once the consecutive-failure count reaches the threshold.
func (cb *circuitBreaker) recordFailure() {
	if cb == nil {
		return
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.probeRunning = false

	if cb.state == circuitHalfOpen {
		cb.trip()
		return
	}
	cb.failures++
	if cb.threshold > 0 && cb.failures >= cb.threshold {
		cb.trip()
	}
}

// trip moves the breaker to the open state and stamps the open time. Callers
// must hold cb.mu.
func (cb *circuitBreaker) trip() {
	cb.state = circuitOpen
	cb.openedAt = cb.now()
}
