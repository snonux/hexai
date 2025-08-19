// Summary: Minimal LSP server over stdio; manages documents, dispatches requests, and tracks stats.
// Not yet reviewed by a human
package lsp

import (
	"bufio"
	"encoding/json"
	"hexai/internal/llm"
	"hexai/internal/logging"
	"io"
	"log"
	"sync"
	"time"
)

// Server implements a minimal LSP over stdio.
type Server struct {
	in               *bufio.Reader
	out              io.Writer
	logger           *log.Logger
	exited           bool
	mu               sync.RWMutex
	docs             map[string]*document
	logContext       bool
	llmClient        llm.Client
	lastInput        time.Time
	maxTokens        int
	contextMode      string
	windowLines      int
	maxContextTokens int
	noDiskIO         bool
    triggerChars     []string
    // If set, used as the LSP coding temperature for all LLM calls
    codingTemperature *float64
    // Concurrency guard: prevent overlapping LLM requests (esp. completions)
    llmBusy bool
	// LLM request stats
	llmReqTotal       int64
	llmSentBytesTotal int64
	llmRespTotal      int64
    llmRespBytesTotal int64
    startTime         time.Time
    // Small LRU cache for recent code completion outputs (keyed by context)
    compCache      map[string]string
    compCacheOrder []string // most-recent at end; cap ~10
}

// ServerOptions collects configuration for NewServer to avoid long parameter lists.
type ServerOptions struct {
    LogContext        bool
    MaxTokens         int
    ContextMode       string
    WindowLines       int
    MaxContextTokens  int

    Client            llm.Client
    TriggerCharacters []string
    CodingTemperature *float64
}

func NewServer(r io.Reader, w io.Writer, logger *log.Logger, opts ServerOptions) *Server {
	s := &Server{in: bufio.NewReader(r), out: w, logger: logger, docs: make(map[string]*document), logContext: opts.LogContext}
	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 500
	}
	s.maxTokens = maxTokens
	contextMode := opts.ContextMode
	if contextMode == "" {
		contextMode = "file-on-new-func"
	}
	windowLines := opts.WindowLines
	if windowLines <= 0 {
		windowLines = 120
	}
	maxContextTokens := opts.MaxContextTokens
	if maxContextTokens <= 0 {
		maxContextTokens = 2000
	}
	s.contextMode = contextMode
	s.windowLines = windowLines
	s.maxContextTokens = maxContextTokens
	
	s.startTime = time.Now()
    s.llmClient = opts.Client
    if len(opts.TriggerCharacters) == 0 {
        // Defaults (explicit space included to allow post-identifier triggers)
        s.triggerChars = []string{".", ":", "/", "_", " "}
    } else {
        s.triggerChars = append([]string{}, opts.TriggerCharacters...)
    }
    s.codingTemperature = opts.CodingTemperature
    s.compCache = make(map[string]string)
    return s
}

// tryStartLLM attempts to mark the LLM as busy. Returns true when it acquired
// the guard; false if another LLM request is already running.
func (s *Server) tryStartLLM() bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.llmBusy {
        return false
    }
    s.llmBusy = true
    return true
}

// endLLM releases the busy guard for LLM requests.
func (s *Server) endLLM() {
    s.mu.Lock()
    s.llmBusy = false
    s.mu.Unlock()
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
			logging.Logf("lsp ", "invalid JSON: %v", err)
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
