// Package policy centralizes the timeout, retry, and circuit-breaker tuning
// values used throughout the LLM HTTP call path. Previously these durations and
// counts were scattered as bare literals across each provider constructor and
// across resilience.go / circuitbreaker.go, which made the operational policy
// hard to discover and easy to drift out of sync.
//
// Consolidating them here gives a single, documented source of truth: anyone
// adjusting how aggressively Hexai times out, retries, or sheds load against an
// upstream LLM API only has to look in one place, and every provider inherits
// the same defaults automatically.
package policy

import "time"

// Request timeouts.
//
// DefaultRequestTimeout is the per-request HTTP client timeout applied to chat
// providers (Anthropic, OpenAI, OpenRouter, Ollama) when the user has not
// configured an explicit RequestTimeout. Thirty seconds is generous enough for
// normal completions yet short enough that a hung connection fails fast rather
// than blocking interactive use indefinitely.
//
// ResearchRequestTimeout applies to the You.com Research provider, whose
// multi-step research pipeline routinely runs much longer than a single chat
// completion. It therefore gets a substantially larger default so legitimate
// long-running research is not cut off prematurely.
const (
	DefaultRequestTimeout  = 30 * time.Second
	ResearchRequestTimeout = 120 * time.Second
)

// DefaultRequestTimeoutSeconds and ResearchRequestTimeoutSeconds expose the
// timeouts as whole seconds, matching the int-seconds shape that provider
// constructors and the user-facing Config.RequestTimeout field use.
const (
	DefaultRequestTimeoutSeconds  = int(DefaultRequestTimeout / time.Second)
	ResearchRequestTimeoutSeconds = int(ResearchRequestTimeout / time.Second)
)

// Retry policy.
//
// These govern how transient LLM HTTP failures (network resets, 5xx, 429 rate
// limits) are retried with exponential backoff plus jitter. The defaults are
// intentionally conservative: a handful of fast attempts that smooth over brief
// upstream blips without hammering the API or stalling interactive use.
//
//   - RetryMaxAttempts: total tries including the first; <=1 disables retries.
//   - RetryBaseDelay:   delay before the first retry; doubles each subsequent attempt.
//   - RetryMaxDelay:    upper bound on any single backoff so growth stays bounded.
//   - RetryJitterFraction: +/- fraction of random jitter to avoid thundering-herd retries.
const (
	RetryMaxAttempts    = 3
	RetryBaseDelay      = 200 * time.Millisecond
	RetryMaxDelay       = 2 * time.Second
	RetryJitterFraction = 0.2
)

// Circuit breaker.
//
// The shared circuit breaker guards the whole LLM HTTP path. After
// CircuitFailureThreshold consecutive transient failures it trips open and
// rejects requests for CircuitCooldown, giving an unhealthy upstream time to
// recover instead of forcing every caller to wait out full retry+timeout
// cycles.
const (
	CircuitFailureThreshold = 5
	CircuitCooldown         = 30 * time.Second
)
