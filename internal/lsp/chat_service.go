package lsp

import (
	"strings"
	"sync"
	"time"
)

// chatService owns the in-editor chat subsystem that used to be inlined on
// Server. It tracks editor input activity (used by the completion debounce
// gate) and reaches back into the Server (via srv) for shared infrastructure
// such as configuration, LLM clients, document access and edit dispatch.
//
// Pulling this out of Server keeps the chat detection/history/command logic
// cohesive and gives the input-activity clock its own small mutex instead of
// piggy-backing on Server.mu.
type chatService struct {
	srv *Server

	activityMu sync.RWMutex
	lastInput  time.Time

	// pendingMu guards pending, the set of chat and inline prompts whose LLM
	// request is still running. Every didChange rescans the document, so
	// without it each keystroke typed while waiting for the model would start
	// another request for the same prompt and insert the answer again.
	pendingMu sync.Mutex
	pending   map[string]bool
}

// newChatService constructs the chat subsystem bound to srv.
func newChatService(srv *Server) *chatService {
	return &chatService{srv: srv}
}

// markActivity records that the editor just sent input. The completion
// debounce gate uses this timestamp to decide how long to wait before issuing
// an LLM request.
func (c *chatService) markActivity() {
	c.activityMu.Lock()
	c.lastInput = time.Now()
	c.activityMu.Unlock()
}

// lastActivity returns the most recent input timestamp (zero if none yet).
func (c *chatService) lastActivity() time.Time {
	c.activityMu.RLock()
	defer c.activityMu.RUnlock()
	return c.lastInput
}

// tryBeginPrompt marks the prompt identified by key as in flight. It returns
// false while a request for the same prompt is running, or after it finished
// while the prompt line is still unchanged in the document.
func (c *chatService) tryBeginPrompt(key string) bool {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if c.pending == nil {
		c.pending = make(map[string]bool)
	}
	if _, busy := c.pending[key]; busy {
		return false
	}
	c.pending[key] = false
	return true
}

// finishPrompt ends the request started by tryBeginPrompt. The key stays
// reserved until prunePrompts sees the prompt line change: on success the
// editor applies the answer asynchronously, so further didChange
// notifications may still show the old prompt line; on failure the user has
// been notified, and retrying on every keystroke would only repeat the error.
// Editing the prompt line (e.g. retyping the trigger) asks again.
func (c *chatService) finishPrompt(key string) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if _, ok := c.pending[key]; ok {
		c.pending[key] = true
	}
}

// prunePrompts releases finished prompts of uri whose prompt line no longer
// exists in lines, so the same question can be asked again later.
func (c *chatService) prunePrompts(uri string, lines []string) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if len(c.pending) == 0 {
		return
	}
	present := make(map[string]struct{}, len(lines))
	for _, ln := range lines {
		present[promptKey(uri, ln)] = struct{}{}
		if tag, ok := c.srv.findInlineTag(ln); ok {
			present[inlineKey(uri, tag.text)] = struct{}{}
		}
	}
	prefix := uri + "\x00"
	for key, finished := range c.pending {
		if !finished || !strings.HasPrefix(key, prefix) {
			continue
		}
		if _, ok := present[key]; !ok {
			delete(c.pending, key)
		}
	}
}

// promptPending reports whether key is in flight or answered and unchanged.
func (c *chatService) promptPending(key string) bool {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	_, ok := c.pending[key]
	return ok
}

// promptKey identifies a prompt by document and the prompt line's text, so the
// key stays stable when lines above it are inserted or removed.
func promptKey(uri, line string) string {
	return uri + "\x00" + strings.TrimSpace(line)
}

// chatSvc returns the chat subsystem, lazily constructing it for the bare
// Server literals used in some tests. Production code always has it wired up
// by NewServer.
func (s *Server) chatSvc() *chatService {
	if s.chat == nil {
		s.chat = newChatService(s)
	}
	return s.chat
}
