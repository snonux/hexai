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
	pending   map[string]*pendingPrompt
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

// pendingPrompt is a chat or inline prompt whose request is running or has
// finished while its prompt line is still in the document.
type pendingPrompt struct {
	finished bool
	// copies is the highest number of lines carrying this prompt seen while
	// it was pending. Identical prompts share one key; when the count drops,
	// one of them was answered or edited and the rest may be asked again.
	copies int
}

// tryBeginPrompt marks the prompt identified by key as in flight; copies is
// the number of lines currently carrying it. It returns false while a request
// for the same prompt is running, or after it finished while the prompt lines
// are still unchanged in the document.
func (c *chatService) tryBeginPrompt(key string, copies int) bool {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if c.pending == nil {
		c.pending = make(map[string]*pendingPrompt)
	}
	if _, busy := c.pending[key]; busy {
		return false
	}
	c.pending[key] = &pendingPrompt{copies: copies}
	return true
}

// finishPrompt ends the request started by tryBeginPrompt. The key stays
// reserved until prunePrompts sees a prompt line change: on success the
// editor applies the answer asynchronously, so further didChange
// notifications may still show the old prompt line; on failure the user has
// been notified, and retrying on every keystroke would only repeat the error.
// Editing the prompt line (e.g. retyping the trigger) asks again.
func (c *chatService) finishPrompt(key string) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if p, ok := c.pending[key]; ok {
		p.finished = true
	}
}

// promptCounts returns, per prompt key, how many of lines carry that prompt
// (as a chat prompt line or an inline prompt tag).
func (c *chatService) promptCounts(uri string, lines []string) map[string]int {
	counts := make(map[string]int, len(lines))
	for _, ln := range lines {
		counts[promptKey(uri, ln)]++
		if tag, ok := c.srv.findInlineTag(ln); ok {
			counts[inlineKey(uri, tag.text)]++
		}
	}
	return counts
}

// prunePrompts releases finished prompts of uri that are carried by fewer
// lines than before (counts as returned by promptCounts), so a prompt that
// was answered or edited, or an identical copy of it elsewhere in the
// document, can be asked again.
func (c *chatService) prunePrompts(uri string, counts map[string]int) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	prefix := uri + "\x00"
	for key, p := range c.pending {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		n := counts[key]
		if n > p.copies {
			p.copies = n
		} else if p.finished && n < p.copies {
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
