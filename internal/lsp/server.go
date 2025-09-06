// Summary: Minimal LSP server over stdio; manages documents, dispatches requests, and tracks stats.
package lsp

import (
    "bufio"
    "encoding/json"
    "codeberg.org/snonux/hexai/internal/llm"
    "codeberg.org/snonux/hexai/internal/logging"
    "io"
    "log"
    "strings"
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
    triggerChars     []string
	// If set, used as the LSP coding temperature for all LLM calls
	codingTemperature *float64
	// LLM request stats
	llmReqTotal       int64
	llmSentBytesTotal int64
	llmRespTotal      int64
	llmRespBytesTotal int64
	startTime         time.Time
	// Small LRU cache for recent code completion outputs (keyed by context)
	compCache      map[string]string
	compCacheOrder []string // most-recent at end; cap ~10
    // Outgoing JSON-RPC id counter for server-initiated requests
    nextID int64
	// Minimum identifier chars required for manual invoke to bypass prefix checks
	manualInvokeMinPrefix int

    // Debounce and throttle settings
    completionDebounce time.Duration
    throttleInterval   time.Duration
    lastLLMCall        time.Time

    // Dispatch table for JSON-RPC methods → handler functions
    handlers map[string]func(Request)

    // Configurable trigger characters
    inlineOpen   string
    inlineClose  string
    chatSuffix   string
    chatPrefixes []string
}

// ServerOptions collects configuration for NewServer to avoid long parameter lists.
type ServerOptions struct {
    LogContext       bool
    MaxTokens        int
    ContextMode      string
    WindowLines      int
    MaxContextTokens int

    Client                llm.Client
    TriggerCharacters     []string
    CodingTemperature     *float64
    ManualInvokeMinPrefix int
    CompletionDebounceMs  int
    CompletionThrottleMs  int

    // Inline/chat triggers
    InlineOpen   string
    InlineClose  string
    ChatSuffix   string
    ChatPrefixes []string
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
		// Defaults (no space to avoid auto-trigger after whitespace)
		s.triggerChars = []string{".", ":", "/", "_", ")", "{"}
	} else {
		s.triggerChars = append([]string{}, opts.TriggerCharacters...)
	}
    s.codingTemperature = opts.CodingTemperature
    s.compCache = make(map[string]string)
    s.manualInvokeMinPrefix = opts.ManualInvokeMinPrefix
    if opts.CompletionDebounceMs > 0 {
        s.completionDebounce = time.Duration(opts.CompletionDebounceMs) * time.Millisecond
    }
    if opts.CompletionThrottleMs > 0 {
        s.throttleInterval = time.Duration(opts.CompletionThrottleMs) * time.Millisecond
    }
    // Trigger character config (with sane defaults if missing)
    if strings.TrimSpace(opts.InlineOpen) == "" { s.inlineOpen = ">" } else { s.inlineOpen = opts.InlineOpen }
    if strings.TrimSpace(opts.InlineClose) == "" { s.inlineClose = ">" } else { s.inlineClose = opts.InlineClose }
    if strings.TrimSpace(opts.ChatSuffix) == "" { s.chatSuffix = ">" } else { s.chatSuffix = opts.ChatSuffix }
    if len(opts.ChatPrefixes) == 0 { s.chatPrefixes = []string{"?","!",":",";"} } else { s.chatPrefixes = append([]string{}, opts.ChatPrefixes...) }

    // Assign package-level inline trigger chars for free helper functions
    if s.inlineOpen != "" { inlineOpenChar = s.inlineOpen[0] }
    if s.inlineClose != "" { inlineCloseChar = s.inlineClose[0] }
    if s.chatSuffix != "" { chatSuffixChar = s.chatSuffix[0] }
    if len(s.chatPrefixes) > 0 { chatPrefixSingles = append([]string{}, s.chatPrefixes...) }
	// Initialize dispatch table
	s.handlers = map[string]func(Request){
		"initialize":              s.handleInitialize,
		"initialized":             func(_ Request) { s.handleInitialized() },
		"shutdown":                s.handleShutdown,
		"exit":                    func(_ Request) { s.handleExit() },
		"textDocument/didOpen":    s.handleDidOpen,
		"textDocument/didChange":  s.handleDidChange,
		"textDocument/didClose":   s.handleDidClose,
		"textDocument/completion": s.handleCompletion,
		"textDocument/codeAction": s.handleCodeAction,
		"codeAction/resolve":      s.handleCodeActionResolve,
		"workspace/executeCommand": s.handleExecuteCommand,
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
