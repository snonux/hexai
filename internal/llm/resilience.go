package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"codeberg.org/snonux/hexai/internal/logging"
)

// errCircuitOpen is returned when the shared circuit breaker is open and is
// rejecting requests during its cooldown window. It is a sentinel so callers
// and tests can detect breaker-induced failures with errors.Is.
var errCircuitOpen = errors.New("llm: circuit breaker open")

// retryPolicy describes how transient LLM HTTP failures are retried. The
// defaults are intentionally conservative: a handful of attempts with
// exponential backoff plus jitter, so that brief upstream blips (network
// resets, 5xx, 429 rate limits) are smoothed over without hammering the API or
// stalling interactive use for long.
//
// Backoff for attempt n (0-indexed) is baseDelay * 2^n, capped at maxDelay,
// with up to +/- jitterFraction random jitter to avoid thundering-herd retries.
type retryPolicy struct {
	maxAttempts    int           // total tries including the first; <=1 disables retries
	baseDelay      time.Duration // delay before the first retry
	maxDelay       time.Duration // upper bound on any single backoff
	jitterFraction float64       // 0..1 fraction of random jitter applied to each delay

	// sleep and rand are injectable seams for deterministic tests. In
	// production they default to a context-aware sleep and package rng.
	sleep     func(ctx context.Context, d time.Duration) error
	randFloat func() float64
}

// defaultRetryPolicy returns the policy used for all LLM HTTP calls. Three
// total attempts (two retries) with 200ms base backoff keeps recovery fast for
// transient errors while staying well within typical request timeouts.
func defaultRetryPolicy() retryPolicy {
	return retryPolicy{
		maxAttempts:    3,
		baseDelay:      200 * time.Millisecond,
		maxDelay:       2 * time.Second,
		jitterFraction: 0.2,
		sleep:          sleepWithContext,
		randFloat:      rand.Float64,
	}
}

// sharedBreaker guards the whole LLM HTTP path. It trips after several
// consecutive transient failures and stays open briefly so a single unhealthy
// upstream does not cause every caller to wait out full retry+timeout cycles.
var sharedBreaker = newCircuitBreaker(5, 30*time.Second)

// sleepWithContext sleeps for d unless ctx is cancelled first, in which case it
// returns ctx.Err(). This keeps backoff waits responsive to cancellation and
// deadlines instead of blocking blindly.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// backoffFor computes the (possibly jittered) delay before the given retry
// attempt index (0 = first retry). It applies exponential growth capped at
// maxDelay, then symmetric jitter of +/- jitterFraction.
func (p retryPolicy) backoffFor(attempt int) time.Duration {
	d := p.baseDelay
	for i := 0; i < attempt && d < p.maxDelay; i++ {
		d *= 2
	}
	if d > p.maxDelay {
		d = p.maxDelay
	}
	if p.jitterFraction > 0 && p.randFloat != nil {
		// Map randFloat()'s [0,1) into [-jitterFraction, +jitterFraction).
		delta := (p.randFloat()*2 - 1) * p.jitterFraction
		d += time.Duration(float64(d) * delta)
	}
	if d < 0 {
		d = 0
	}
	return d
}

// shouldRetryStatus reports whether an HTTP status code represents a transient
// server-side condition worth retrying. We retry 429 (rate limited) and 5xx
// (server errors). We deliberately do NOT retry other 4xx codes: those are
// client errors (bad request, auth, not found) that will fail identically on
// retry and must surface immediately.
func shouldRetryStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

// doJSONRequestResilient wraps doJSONRequestOnce with retry-with-backoff and a
// shared circuit breaker. It retries transient network errors and retryable
// HTTP statuses (429/5xx) per the supplied policy, while respecting context
// cancellation/deadlines throughout. Non-retryable responses (including 4xx and
// any success) are returned to the caller unread so providers can parse the
// body as before.
func doJSONRequestResilient(ctx context.Context, httpClient *http.Client, url string, body []byte, headers map[string]string, accept string, policy retryPolicy, breaker *circuitBreaker) (*http.Response, error) {
	if !breaker.Allow() {
		logging.Logf("llm/resilience ", "%scircuit open: rejecting request to %s%s", logging.AnsiRed, url, logging.AnsiBase)
		return nil, errCircuitOpen
	}

	attempts := policy.maxAttempts
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		resp, retryable, err := attemptJSONRequest(ctx, httpClient, url, body, headers, accept)
		if err == nil && !retryable {
			breaker.recordSuccess()
			return resp, nil
		}
		lastErr = err
		// Sleep before the next attempt unless this was the last one.
		if attempt < attempts-1 {
			if werr := waitBeforeRetry(ctx, policy, attempt, url, err); werr != nil {
				breaker.recordFailure()
				return nil, werr
			}
		}
	}

	// All attempts exhausted on transient failures: count it against the breaker.
	breaker.recordFailure()
	if lastErr == nil {
		lastErr = fmt.Errorf("llm: request to %s failed after %d attempts", url, attempts)
	}
	return nil, lastErr
}

// attemptJSONRequest performs a single HTTP attempt. It returns retryable=true
// when the caller should retry: either a network/transport error, or a
// retryable status (429/5xx) whose body is drained and closed so the connection
// can be reused. A non-retryable response is returned with its body intact.
func attemptJSONRequest(ctx context.Context, httpClient *http.Client, url string, body []byte, headers map[string]string, accept string) (resp *http.Response, retryable bool, err error) {
	resp, err = doJSONRequestOnce(ctx, httpClient, url, body, headers, accept)
	if err != nil {
		// Context cancellation is not a transient condition: do not retry.
		if ctx.Err() != nil {
			return nil, false, err
		}
		return nil, true, err
	}
	if shouldRetryStatus(resp.StatusCode) {
		// Drain and close so the keep-alive connection can be reused, then
		// signal a retry via a synthetic error for logging/diagnostics.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return nil, true, fmt.Errorf("llm: retryable status %d from %s", resp.StatusCode, url)
	}
	return resp, false, nil
}

// waitBeforeRetry logs the impending retry and sleeps for the policy backoff,
// returning early if the context is cancelled during the wait.
func waitBeforeRetry(ctx context.Context, policy retryPolicy, attempt int, url string, cause error) error {
	delay := policy.backoffFor(attempt)
	logging.Logf("llm/resilience ", "retry %d for %s after %s (cause: %v)", attempt+1, url, delay, cause)
	sleep := policy.sleep
	if sleep == nil {
		sleep = sleepWithContext
	}
	return sleep(ctx, delay)
}
