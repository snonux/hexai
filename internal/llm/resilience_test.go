package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// testPolicy returns a retry policy with deterministic, instant backoff and no
// jitter so tests are fast and reproducible.
func testPolicy(maxAttempts int) retryPolicy {
	return retryPolicy{
		maxAttempts:    maxAttempts,
		baseDelay:      time.Millisecond,
		maxDelay:       time.Millisecond,
		jitterFraction: 0,
		sleep:          func(ctx context.Context, d time.Duration) error { return ctx.Err() },
		randFloat:      func() float64 { return 0.5 },
	}
}

func TestShouldRetryStatus(t *testing.T) {
	cases := map[int]bool{
		200: false, 201: false,
		400: false, 401: false, 404: false,
		429: true,
		500: true, 502: true, 503: true,
	}
	for status, want := range cases {
		if got := shouldRetryStatus(status); got != want {
			t.Errorf("shouldRetryStatus(%d)=%v want %v", status, got, want)
		}
	}
}

func TestResilient_RetriesThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, "boom")
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	resp, err := doJSONRequestResilient(context.Background(), srv.Client(), srv.URL, []byte("{}"), nil, "", testPolicy(3), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 calls, got %d", got)
	}
}

func TestResilient_NoRetryOn4xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	resp, err := doJSONRequestResilient(context.Background(), srv.Client(), srv.URL, []byte("{}"), nil, "", testPolicy(3), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("4xx must not be retried: got %d calls", got)
	}
}

func TestResilient_ExhaustsRetries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := doJSONRequestResilient(context.Background(), srv.Client(), srv.URL, []byte("{}"), nil, "", testPolicy(3), nil)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestResilient_NetworkErrorRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // closed server: connection attempts fail at the transport layer

	_, err := doJSONRequestResilient(context.Background(), http.DefaultClient, url, []byte("{}"), nil, "", testPolicy(2), nil)
	if err == nil {
		t.Fatal("expected network error")
	}
}

func TestResilient_ContextCancelStopsRetries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel during the backoff sleep after the first failed attempt.
	policy := testPolicy(5)
	policy.sleep = func(c context.Context, d time.Duration) error {
		cancel()
		return c.Err()
	}

	_, err := doJSONRequestResilient(ctx, srv.Client(), srv.URL, []byte("{}"), nil, "", policy, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 attempt before cancel, got %d", got)
	}
}

func TestResilient_AlreadyCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := doJSONRequestResilient(ctx, srv.Client(), srv.URL, []byte("{}"), nil, "", testPolicy(3), nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestResilient_CircuitOpenRejects(t *testing.T) {
	cb := newCircuitBreaker(1, time.Hour)
	cb.recordFailure() // trips immediately (threshold 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be reached while circuit is open")
	}))
	defer srv.Close()

	_, err := doJSONRequestResilient(context.Background(), srv.Client(), srv.URL, []byte("{}"), nil, "", testPolicy(3), cb)
	if !errors.Is(err, errCircuitOpen) {
		t.Fatalf("expected errCircuitOpen, got %v", err)
	}
}

func TestResilient_BreakerTripsAfterExhaustedRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cb := newCircuitBreaker(1, time.Hour) // a single exhausted run trips it
	if _, err := doJSONRequestResilient(context.Background(), srv.Client(), srv.URL, []byte("{}"), nil, "", testPolicy(2), cb); err == nil {
		t.Fatal("expected error")
	}
	// Next call must be rejected by the now-open breaker.
	if _, err := doJSONRequestResilient(context.Background(), srv.Client(), srv.URL, []byte("{}"), nil, "", testPolicy(2), cb); !errors.Is(err, errCircuitOpen) {
		t.Fatalf("expected errCircuitOpen on second call, got %v", err)
	}
}

func TestBackoffFor_ExponentialCappedNoJitter(t *testing.T) {
	p := retryPolicy{baseDelay: 100 * time.Millisecond, maxDelay: 400 * time.Millisecond}
	want := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		400 * time.Millisecond, // capped
	}
	for i, w := range want {
		if got := p.backoffFor(i); got != w {
			t.Errorf("backoffFor(%d)=%s want %s", i, got, w)
		}
	}
}

func TestBackoffFor_JitterWithinBounds(t *testing.T) {
	p := retryPolicy{
		baseDelay:      100 * time.Millisecond,
		maxDelay:       time.Second,
		jitterFraction: 0.5,
		randFloat:      func() float64 { return 1.0 }, // max positive jitter
	}
	got := p.backoffFor(0)
	// 100ms + 50% = 150ms.
	if got != 150*time.Millisecond {
		t.Fatalf("got %s want 150ms", got)
	}
	p.randFloat = func() float64 { return 0.0 } // max negative jitter
	if got := p.backoffFor(0); got != 50*time.Millisecond {
		t.Fatalf("got %s want 50ms", got)
	}
}

func TestSleepWithContext_ReturnsOnDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepWithContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled, got %v", err)
	}
	// Zero delay returns immediately with ctx.Err() (nil here).
	if err := sleepWithContext(context.Background(), 0); err != nil {
		t.Fatalf("expected nil for zero delay, got %v", err)
	}
	// Positive delay elapses normally.
	if err := sleepWithContext(context.Background(), time.Millisecond); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResilient_DefaultPolicyDisabledRetriesWhenSingleAttempt(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// maxAttempts <= 1 means no retries: exactly one attempt.
	if _, err := doJSONRequestResilient(context.Background(), srv.Client(), srv.URL, []byte("{}"), nil, "", testPolicy(0), nil); err == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected exactly 1 attempt, got %d", got)
	}
}
