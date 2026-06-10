package lsp

import (
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

// chatSvc returns the chat subsystem, lazily constructing it for the bare
// Server literals used in some tests. Production code always has it wired up
// by NewServer.
func (s *Server) chatSvc() *chatService {
	if s.chat == nil {
		s.chat = newChatService(s)
	}
	return s.chat
}
