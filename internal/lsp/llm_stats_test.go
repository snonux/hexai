package lsp

import "testing"

func TestIncSentCounters(t *testing.T) {
	s := newTestServer()
	s.incSentCounters(10)
	s.incSentCounters(20)
	if got := s.llmReqTotal.Load(); got != 2 {
		t.Fatalf("expected reqTotal=2, got %d", got)
	}
	if got := s.llmSentBytesTotal.Load(); got != 30 {
		t.Fatalf("expected sentBytesTotal=30, got %d", got)
	}
}

func TestIncRecvCounters(t *testing.T) {
	s := newTestServer()
	s.incRecvCounters(15)
	if got := s.llmRespTotal.Load(); got != 1 {
		t.Fatalf("expected respTotal=1, got %d", got)
	}
	if got := s.llmRespBytesTotal.Load(); got != 15 {
		t.Fatalf("expected respBytesTotal=15, got %d", got)
	}
}

func TestLogLLMStats_ZeroCounters(t *testing.T) {
	s := newTestServer()
	// Should not panic with zero counters (divide-by-zero guard).
	s.logLLMStats("model")
}

func TestLogLLMStats_WithData(t *testing.T) {
	s := newTestServer()
	s.incSentCounters(10)
	s.incRecvCounters(20)
	// Should not panic and exercises the averaging paths.
	s.logLLMStats("model")
}
