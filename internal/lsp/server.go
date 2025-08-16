package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"hexai/internal"
	"hexai/internal/llm"
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
	Label            string    `json:"label"`
	Kind             int       `json:"kind,omitempty"`
	Detail           string    `json:"detail,omitempty"`
	InsertText       string    `json:"insertText,omitempty"`
	InsertTextFormat int       `json:"insertTextFormat,omitempty"`
	FilterText       string    `json:"filterText,omitempty"`
	TextEdit         *TextEdit `json:"textEdit,omitempty"`
	SortText         string    `json:"sortText,omitempty"`
	Documentation    string    `json:"documentation,omitempty"`
}

// Server implements a minimal LSP over stdio.
type Server struct {
	in         *bufio.Reader
	out        io.Writer
	logger     *log.Logger
	exited     bool
	mu         sync.RWMutex
	docs       map[string]*document
	logContext bool
	llmClient  llm.Client
	lastInput  time.Time
}

func NewServer(r io.Reader, w io.Writer, logger *log.Logger, logContext bool) *Server {
	s := &Server{in: bufio.NewReader(r), out: w, logger: logger, docs: make(map[string]*document), logContext: logContext}
	if c, err := llm.NewDefault(logger); err != nil {
		// Keep running without LLM; completions will be basic.
		s.logger.Printf("llm disabled: %v", err)
	} else {
		s.llmClient = c
	}
	return s
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
			s.markActivity()
		}
	case "textDocument/didChange":
		var p DidChangeTextDocumentParams
		if err := json.Unmarshal(req.Params, &p); err == nil {
			if len(p.ContentChanges) > 0 {
				s.setDocument(p.TextDocument.URI, p.ContentChanges[len(p.ContentChanges)-1].Text)
			}
			s.markActivity()
		}
	case "textDocument/didClose":
		var p DidCloseTextDocumentParams
		if err := json.Unmarshal(req.Params, &p); err == nil {
			s.deleteDocument(p.TextDocument.URI)
			s.markActivity()
		}
	case "textDocument/completion":
		var p CompletionParams
		var docStr string
		if err := json.Unmarshal(req.Params, &p); err == nil {
			above, current, below, funcCtx := s.lineContext(p.TextDocument.URI, p.Position)
			docStr = fmt.Sprintf("file: %s\nline: %d\nabove: %s\ncurrent: %s\nbelow: %s\nfunction: %s", p.TextDocument.URI, p.Position.Line, trimLen(above), trimLen(current), trimLen(below), trimLen(funcCtx))
			if s.logContext {
				s.logger.Printf("completion ctx uri=%s line=%d char=%d above=%q current=%q below=%q function=%q",
					p.TextDocument.URI, p.Position.Line, p.Position.Character, trimLen(above), trimLen(current), trimLen(below), trimLen(funcCtx))
			}
			// Previously: gated LLM calls until 1s idle. Removed to complete as you type.
			// Try LLM-backed suggestion if available (always, no idle gating)
			if s.llmClient != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
				defer cancel()
				// Tailor prompt if inside a Go function parameter list
				inParams := false
				if strings.Contains(current, "func ") {
					open := strings.Index(current, "(")
					close := strings.Index(current, ")")
					if open >= 0 && p.Position.Character > open && (close == -1 || p.Position.Character <= close) {
						inParams = true
					}
				}
				sysPrompt := "You are a terse code completion engine. Return only the code to insert, no surrounding prose or backticks."
				userPrompt := fmt.Sprintf("Provide the next likely code to insert at the cursor.\nFile: %s\nFunction/context: %s\nAbove line: %s\nCurrent line (cursor at character %d): %s\nBelow line: %s\nOnly return the completion snippet.", p.TextDocument.URI, funcCtx, above, p.Position.Character, current, below)
				if inParams {
					sysPrompt = "You are a terse Go code completion engine for function signatures. Return only the parameter list contents (without parentheses), no braces, no prose. Prefer idiomatic names and types."
					userPrompt = fmt.Sprintf("Cursor is inside the function parameter list. Suggest only the parameter list (no parentheses).\nFunction line: %s\nCurrent line (cursor at %d): %s", funcCtx, p.Position.Character, current)
				}
				messages := []llm.Message{
					{Role: "system", Content: sysPrompt},
					{Role: "user", Content: userPrompt},
				}
				// keep completions small by default
				text, err := s.llmClient.Chat(ctx, messages, llm.WithMaxTokens(96), llm.WithTemperature(0.2))
				if err == nil && strings.TrimSpace(text) != "" {
					cleaned := strings.TrimSpace(text)
					var te *TextEdit
					var filter string
					if inParams {
						// Replace inside the parentheses
						open := strings.Index(current, "(")
						close := strings.Index(current, ")")
						if open >= 0 {
							left := open + 1
							right := len(current)
							if close >= 0 && close >= left {
								right = close
							}
							if p.Position.Character < right {
								right = p.Position.Character
							}
							te = &TextEdit{Range: Range{Start: Position{Line: p.Position.Line, Character: left}, End: Position{Line: p.Position.Line, Character: right}}, NewText: cleaned}
							if left >= 0 && right >= left && right <= len(current) {
								filter = strings.TrimLeft(current[left:right], " \t")
							}
						}
					}
					if te == nil {
						// compute word start for replacement
						startChar := p.Position.Character
						if startChar > len(current) {
							startChar = len(current)
						}
						for startChar > 0 {
							ch := current[startChar-1]
							if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
								startChar--
								continue
							}
							break
						}
						te = &TextEdit{Range: Range{Start: Position{Line: p.Position.Line, Character: startChar}, End: Position{Line: p.Position.Line, Character: p.Position.Character}}, NewText: cleaned}
						filter = strings.TrimLeft(current[startChar:p.Position.Character], " \t")
					}
					// Choose a label that starts with the current prefix when possible so the client doesn't filter it out.
					label := trimLen(firstLine(cleaned))
					if filter != "" && !strings.HasPrefix(strings.ToLower(label), strings.ToLower(filter)) {
						label = filter
					}
					items := []CompletionItem{{
						Label:            label,
						Kind:             1,
						Detail:           "OpenAI completion",
						InsertTextFormat: 1,
						FilterText:       strings.TrimLeft(filter, " \t"),
						TextEdit:         te,
						SortText:         "0000",
						Documentation:    docStr,
					}}
					s.reply(req.ID, CompletionList{IsIncomplete: false, Items: items}, nil)
					return
				}
				if err != nil {
					s.logger.Printf("llm completion error: %v", err)
				}
			}
		}
		// Fallback basic/dummy completion
		items := []CompletionItem{{
			Label:         "hexai-complete",
			Kind:          1,
			Detail:        "dummy completion",
			InsertText:    "hexai",
			SortText:      "9999",
			Documentation: docStr,
		}}
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

func (s *Server) markActivity() {
	s.mu.Lock()
	s.lastInput = time.Now()
	s.mu.Unlock()
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

// Range defines a text range in a document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// TextEdit represents a textual edit applicable to a document.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
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

func firstLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}
