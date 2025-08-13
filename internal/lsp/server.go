package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"hexai/internal"
	"io"
	"log"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"sync"
    "time"
)

// JSON-RPC 2.0 structures (minimal)
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RespError      `json:"error,omitempty"`
}

type RespError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// LSP responses (subset)
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   *ServerInfo        `json:"serverInfo,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type ServerCapabilities struct {
	TextDocumentSync   any                `json:"textDocumentSync,omitempty"`
	CompletionProvider *CompletionOptions `json:"completionProvider,omitempty"`
}

type CompletionOptions struct {
	ResolveProvider   bool     `json:"resolveProvider,omitempty"`
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

type CompletionItem struct {
	Label         string `json:"label"`
	Kind          int    `json:"kind,omitempty"`
	Detail        string `json:"detail,omitempty"`
	InsertText    string `json:"insertText,omitempty"`
	SortText      string `json:"sortText,omitempty"`
	Documentation string `json:"documentation,omitempty"`
}

// Server implements a minimal LSP over stdio.
type Server struct {
    in     *bufio.Reader
    out    io.Writer
    logger *log.Logger
    exited bool
    mu     sync.RWMutex
    docs   map[string]*document
    logContext bool
    lastChange map[string]time.Time
}

func NewServer(r io.Reader, w io.Writer, logger *log.Logger, logContext bool) *Server {
    return &Server{in: bufio.NewReader(r), out: w, logger: logger, docs: make(map[string]*document), logContext: logContext, lastChange: make(map[string]time.Time)}
}

func (s *Server) Run() error {
	for {
		body, err := s.readMessage()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		var req Request
		if err := json.Unmarshal(body, &req); err != nil {
			s.logger.Printf("invalid JSON: %v", err)
			continue
		}
		if req.Method == "" {
			// A response from client; ignore
			continue
		}
		go s.handle(req)
		if s.exited {
			return nil
		}
	}
}

func (s *Server) handle(req Request) {
	switch req.Method {
	case "initialize":
		res := InitializeResult{
			Capabilities: ServerCapabilities{
				// 1 = TextDocumentSyncKindFull
				TextDocumentSync: 1,
				CompletionProvider: &CompletionOptions{
					ResolveProvider:   false,
					TriggerCharacters: []string{".", ":", "/", "_"},
				},
			},
			ServerInfo: &ServerInfo{Name: "hexai", Version: internal.Version},
		}
		s.reply(req.ID, res, nil)
	case "initialized":
		// Notification; no response
		s.logger.Println("client initialized")
	case "shutdown":
		s.reply(req.ID, nil, nil)
	case "exit":
		s.exited = true
		// No response per spec.
		os.Exit(0)
    case "textDocument/didOpen":
        var p DidOpenTextDocumentParams
        if err := json.Unmarshal(req.Params, &p); err == nil {
            s.setDocument(p.TextDocument.URI, p.TextDocument.Text)
            s.setLastChange(p.TextDocument.URI, time.Now())
        }
    case "textDocument/didChange":
        var p DidChangeTextDocumentParams
        if err := json.Unmarshal(req.Params, &p); err == nil {
            if len(p.ContentChanges) > 0 {
                s.setDocument(p.TextDocument.URI, p.ContentChanges[len(p.ContentChanges)-1].Text)
                s.setLastChange(p.TextDocument.URI, time.Now())
            }
        }
    case "textDocument/didClose":
        var p DidCloseTextDocumentParams
        if err := json.Unmarshal(req.Params, &p); err == nil {
            s.deleteDocument(p.TextDocument.URI)
            s.clearLastChange(p.TextDocument.URI)
        }
    case "textDocument/completion":
        var p CompletionParams
        var docStr string
        allowed := true
        if err := json.Unmarshal(req.Params, &p); err == nil {
            above, current, below, funcCtx := s.lineContext(p.TextDocument.URI, p.Position)
            docStr = fmt.Sprintf("file: %s\nline: %d\nabove: %s\ncurrent: %s\nbelow: %s\nfunction: %s", p.TextDocument.URI, p.Position.Line, trimLen(above), trimLen(current), trimLen(below), trimLen(funcCtx))
            if s.logContext {
                s.logger.Printf("completion ctx uri=%s line=%d char=%d above=%q current=%q below=%q function=%q",
                    p.TextDocument.URI, p.Position.Line, p.Position.Character, trimLen(above), trimLen(current), trimLen(below), trimLen(funcCtx))
            }
            // Apply gating: require space before cursor AND >=2s idle since last change
            if !s.prevCharIsSpace(p.TextDocument.URI, p.Position) {
                allowed = false
            }
            if since := s.sinceLastChange(p.TextDocument.URI); since >= 0 && since < 2*time.Second {
                allowed = false
            }
        }
        var items []CompletionItem
        if allowed {
            items = []CompletionItem{{
                Label:         "hexai-complete",
                Kind:          14,
                Detail:        "dummy completion",
                InsertText:    "hexai",
                SortText:      "0000",
                Documentation: docStr,
            }}
        } else if s.logContext {
            s.logger.Printf("completion gated: uri=%s allowed=%v idle=%v spaceBefore=%v",
                p.TextDocument.URI, allowed, s.sinceLastChange(p.TextDocument.URI), s.prevCharIsSpace(p.TextDocument.URI, p.Position))
        }
        s.reply(req.ID, CompletionList{IsIncomplete: false, Items: items}, nil)
	default:
		// Unknown method; reply with Method Not Found for requests that have an ID.
		if len(req.ID) != 0 {
			s.reply(req.ID, nil, &RespError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)})
		}
	}
}

func (s *Server) reply(id json.RawMessage, result any, err *RespError) {
	resp := Response{JSONRPC: "2.0", ID: id, Result: result, Error: err}
	s.writeMessage(resp)
}

func (s *Server) readMessage() ([]byte, error) {
	tp := textproto.NewReader(s.in)
	var contentLength int
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return nil, err
		}
		if line == "" { // end of headers
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(parts[0]))
		val := strings.TrimSpace(parts[1])
		switch key {
		case "content-length":
			n, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %v", err)
			}
			contentLength = n
		}
	}
	if contentLength <= 0 {
		return nil, fmt.Errorf("missing or invalid Content-Length")
	}
	buf := make([]byte, contentLength)
	if _, err := io.ReadFull(s.in, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (s *Server) writeMessage(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		s.logger.Printf("marshal error: %v", err)
		return
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := io.WriteString(s.out, header); err != nil {
		s.logger.Printf("write header error: %v", err)
		return
	}
	if _, err := s.out.Write(data); err != nil {
		s.logger.Printf("write body error: %v", err)
		return
	}
}

// --- Document store and helpers ---

type document struct {
	uri   string
	text  string
	lines []string
}

func (s *Server) setDocument(uri, text string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.docs[uri] = &document{uri: uri, text: text, lines: splitLines(text)}
}

func (s *Server) deleteDocument(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, uri)
}

func (s *Server) getDocument(uri string) *document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.docs[uri]
}

func splitLines(sx string) []string {
    sx = strings.ReplaceAll(sx, "\r\n", "\n")
    return strings.Split(sx, "\n")
}

// LSP param types (subset)
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId,omitempty"`
	Version    int    `json:"version,omitempty"`
	Text       string `json:"text"`
}

type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version,omitempty"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type TextDocumentContentChangeEvent struct {
	Range       any    `json:"range,omitempty"`
	RangeLength int    `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      any                    `json:"context,omitempty"`
}

func (s *Server) lineContext(uri string, pos Position) (above, current, below, funcCtx string) {
    d := s.getDocument(uri)
    if d == nil || len(d.lines) == 0 {
        return "", "", "", ""
    }
	idx := pos.Line
	if idx < 0 {
		idx = 0
	}
	if idx >= len(d.lines) {
		idx = len(d.lines) - 1
	}
	current = d.lines[idx]
	if idx-1 >= 0 {
		above = d.lines[idx-1]
	}
	if idx+1 < len(d.lines) {
		below = d.lines[idx+1]
	}
	for i := idx; i >= 0; i-- {
		line := strings.TrimSpace(d.lines[i])
		if hasAny(line, []string{"func ", "def ", "class ", "fn ", "procedure ", "sub "}) {
			funcCtx = line
			break
		}
	}
    return
}

func hasAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func trimLen(s string) string {
    s = strings.TrimSpace(s)
    if len(s) > 200 {
        return s[:200] + "…"
    }
    return s
}

// --- Gating helpers ---
func (s *Server) setLastChange(uri string, t time.Time) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.lastChange[uri] = t
}

func (s *Server) clearLastChange(uri string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    delete(s.lastChange, uri)
}

func (s *Server) sinceLastChange(uri string) time.Duration {
    s.mu.RLock()
    t, ok := s.lastChange[uri]
    s.mu.RUnlock()
    if !ok {
        return -1
    }
    return time.Since(t)
}

func (s *Server) prevCharIsSpace(uri string, pos Position) bool {
    d := s.getDocument(uri)
    if d == nil {
        return false
    }
    if pos.Line < 0 || pos.Line >= len(d.lines) {
        return false
    }
    line := d.lines[pos.Line]
    // Convert to runes to be safe with multibyte
    r := []rune(line)
    if pos.Character <= 0 || pos.Character > len(r) {
        return false
    }
    return r[pos.Character-1] == ' '
}
