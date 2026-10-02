package lsp

import (
	"context"
	"sync"
	"time"
)

// completionRequestTimeout bounds a single completion request, LLM calls
// included.
const completionRequestTimeout = 12 * time.Second

// completionService owns the entire code-completion subsystem that used to be
// scattered across Server. It bundles the completion cache/throttle state
// (completionState) with the request-handling logic, and reaches back into the
// Server (via srv) for shared infrastructure such as configuration, LLM
// clients, document access and stats counters.
//
// Splitting this out of Server keeps the completion logic cohesive and lets the
// completionState mutex stay private to this subsystem, independent of
// Server.mu (the two locks are never held simultaneously).
type completionService struct {
	srv *Server
	completionState

	// activeMu guards active: the cancel func of the running completion
	// request per document URI. A newer request for the same document cancels
	// the older one, whose result the client would discard anyway.
	activeMu sync.Mutex
	active   map[string]*activeCompletion
}

type activeCompletion struct {
	cancel context.CancelFunc
}

// newCompletionService constructs the completion subsystem bound to srv.
func newCompletionService(srv *Server) *completionService {
	return &completionService{
		srv:             srv,
		completionState: newCompletionState(),
	}
}

// beginCompletion registers a completion request for uri, cancelling any
// older request for the same document still waiting on debounce or the LLM.
// The returned done func must be called when the request has been answered;
// pass background=true when LLM work continues after the reply (the
// return-first mode collecting the remaining backends), which then keeps the
// context alive until that work's own timeout has passed.
func (cs *completionService) beginCompletion(uri string) (context.Context, func(background bool)) {
	ctx, cancel := context.WithCancel(cs.srv.baseContext())
	entry := &activeCompletion{cancel: cancel}
	cs.activeMu.Lock()
	if cs.active == nil {
		cs.active = make(map[string]*activeCompletion)
	}
	if prev := cs.active[uri]; prev != nil {
		prev.cancel()
	}
	cs.active[uri] = entry
	cs.activeMu.Unlock()
	return ctx, func(background bool) {
		cs.activeMu.Lock()
		if cs.active[uri] == entry {
			delete(cs.active, uri)
		}
		cs.activeMu.Unlock()
		if background {
			time.AfterFunc(completionRequestTimeout+time.Second, cancel)
			return
		}
		cancel()
	}
}

// waitForThrottle gates completion (and chat) LLM calls using the configured
// throttle interval. It lives here because the throttle clock is part of the
// completion state.
func (cs *completionService) waitForThrottle(ctx context.Context) bool {
	return cs.completionState.waitForThrottle(ctx, cs.srv.completionThrottle())
}

// completionSvc returns the completion subsystem, lazily constructing it for
// the bare Server literals used in some tests. Production code always has it
// wired up by NewServer.
func (s *Server) completionSvc() *completionService {
	if s.completion == nil {
		s.completion = newCompletionService(s)
	}
	return s.completion
}

// --- Server delegation shims ---------------------------------------------
// These keep existing Server call sites (and tests) working while the real
// state and behavior live on completionService.

func (s *Server) storePendingCompletion(key string, items []CompletionItem) {
	s.completionSvc().storePendingCompletion(key, items)
}

func (s *Server) setCompletionsDisabled(disabled bool) bool {
	return s.completionSvc().setCompletionsDisabled(disabled)
}

func (s *Server) completionDisabled() bool {
	return s.completionSvc().completionDisabled()
}

func (s *Server) takePendingCompletion(key string) []CompletionItem {
	return s.completionSvc().takePendingCompletion(key)
}

func (s *Server) completionCacheGet(key string) (string, bool) {
	return s.completionSvc().cacheGet(key)
}

func (s *Server) completionCachePut(key, value string) {
	s.completionSvc().cachePut(key, value)
}

func (s *Server) waitForThrottle(ctx context.Context) bool {
	return s.completionSvc().waitForThrottle(ctx)
}
