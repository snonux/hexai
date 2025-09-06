package lsp

import "testing"

func TestLogLLMStats_CoversCounters(t *testing.T) {
	s := newTestServer()
	s.incSentCounters(10)
	s.incRecvCounters(20)
	s.logLLMStats() // just ensure it does not panic and executes
}
