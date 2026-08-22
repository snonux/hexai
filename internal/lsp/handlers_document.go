// Document open/change/close handlers plus the shared client-edit transport
// helpers (workspace/applyEdit, window/showDocument). The in-editor chat logic
// that used to live here was extracted into chatService (chat_handlers.go);
// these handlers now delegate chat detection to s.chatSvc().

package lsp

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/snonux/hexai/internal/logging"
)

func (s *Server) handleDidOpen(req Request) {
	var p DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params, &p); err == nil {
		s.setDocument(p.TextDocument.URI, p.TextDocument.Text)
		s.markActivity()
		// Log when an ignored file is opened (document still stored for editor sync)
		if ignored, reason := s.isFileIgnored(p.TextDocument.URI); ignored {
			logging.Logf("lsp ", "file opened (ignored): %s (%s)", p.TextDocument.URI, reason)
		}
	}
}

func (s *Server) handleDidChange(req Request) {
	var p DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params, &p); err == nil {
		if len(p.ContentChanges) > 0 {
			s.setDocument(p.TextDocument.URI, p.ContentChanges[len(p.ContentChanges)-1].Text)
		}
		s.markActivity()
		// Detect in-editor chat trigger lines and respond inline.
		s.chatSvc().detectAndHandleChat(p.TextDocument.URI)
	}
}

func (s *Server) handleDidClose(req Request) {
	var p DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params, &p); err == nil {
		s.deleteDocument(p.TextDocument.URI)
		s.markActivity()
	}
}

// docBeforeAfter returns the full document text split at the given position.
// The returned strings are the text before the cursor (inclusive of anything
// left of the position) and the text after the cursor.
func (s *Server) docBeforeAfter(uri string, pos Position) (string, string) {
	d := s.getDocument(uri)
	if d == nil {
		return "", ""
	}
	// Clamp indices
	line := pos.Line
	if line < 0 {
		line = 0
	}
	if line >= len(d.lines) {
		line = len(d.lines) - 1
	}
	col := pos.Character
	if col < 0 {
		col = 0
	}
	if col > len(d.lines[line]) {
		col = len(d.lines[line])
	}
	// Build before
	var b strings.Builder
	for i := 0; i < line; i++ {
		b.WriteString(d.lines[i])
		b.WriteByte('\n')
	}
	b.WriteString(d.lines[line][:col])
	before := b.String()
	// Build after
	var a strings.Builder
	a.WriteString(d.lines[line][col:])
	for i := line + 1; i < len(d.lines); i++ {
		a.WriteByte('\n')
		a.WriteString(d.lines[i])
	}
	return before, a.String()
}

// clientApplyEdit sends a workspace/applyEdit request to the client.
func (s *Server) clientApplyEdit(label string, edit WorkspaceEdit) {
	params := ApplyWorkspaceEditParams{Label: label, Edit: edit}
	b, err := json.Marshal(params)
	if err != nil {
		logging.Logf("lsp ", "clientApplyEdit: marshal error: %v", err)
		return
	}
	id := s.nextReqID()
	req := Request{JSONRPC: "2.0", ID: id, Method: "workspace/applyEdit"}
	req.Params = b
	s.writeMessage(req)
}

// nextReqID returns a unique json.RawMessage id for server-initiated requests.
func (s *Server) nextReqID() json.RawMessage {
	s.mu.Lock()
	s.nextID++
	idNum := s.nextID
	s.mu.Unlock()
	b, _ := json.Marshal(idNum)
	return b
}

// clientShowDocument asks the client to open/focus a document and select a range.
func (s *Server) clientShowDocument(uri string, sel *Range) {
	var params struct {
		URI       string `json:"uri"`
		External  bool   `json:"external,omitempty"`
		TakeFocus bool   `json:"takeFocus,omitempty"`
		Selection *Range `json:"selection,omitempty"`
	}
	params.URI = uri
	params.TakeFocus = true
	params.Selection = sel
	b, err := json.Marshal(params)
	if err != nil {
		logging.Logf("lsp ", "clientShowDocument: marshal error: %v", err)
		return
	}
	id := s.nextReqID()
	req := Request{JSONRPC: "2.0", ID: id, Method: "window/showDocument"}
	req.Params = b
	s.writeMessage(req)
}

// deferShowDocument schedules a showDocument after a short delay to allow the client
// time to apply any pending edits (e.g., create the file before focusing it).
// The goroutine respects s.serverCtx so it won't write after shutdown.
func (s *Server) deferShowDocument(uri string, sel Range) {
	ctx := s.serverCtx
	if ctx == nil {
		// Fallback for tests that don't set a server context.
		ctx = context.Background()
	}
	s.inflight.Add(1)
	go func() {
		defer s.inflight.Done()
		timer := time.NewTimer(120 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
			s.clientShowDocument(uri, &sel)
		case <-ctx.Done():
		}
	}()
}
