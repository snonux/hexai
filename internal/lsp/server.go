// Package lsp provides a minimal LSP server over stdio; manages documents, dispatches requests, and tracks stats.
package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/ignore"
	"github.com/snonux/hexai/internal/llm"
	"github.com/snonux/hexai/internal/llmutils"
	"github.com/snonux/hexai/internal/logging"
	"github.com/snonux/hexai/internal/runtimeconfig"
)

// Server implements a minimal LSP over stdio.
type Server struct {
	in           *bufio.Reader
	out          io.Writer
	outMu        sync.Mutex
	logger       *log.Logger
	serverCtx    context.Context
	serverCancel context.CancelFunc
	statusSink   StatusSink
	exited       atomic.Bool
	inflight     sync.WaitGroup // tracks background goroutines (inline prompt, chat, etc.)
	// mu protects docs, cfg, logContext, configLoadOpts, and nextID.
	// It is never held while completionState.stateMu (owned by the completion
	// service) is held, and vice versa, so there is no lock ordering concern
	// between them.
	mu          sync.RWMutex
	docs        map[string]*document
	logContext  bool
	configStore *runtimeconfig.Store
	cfg         appconfig.App
	codeActionSubsystem
	llmStatsSubsystem
	// chat and completion own the two large feature subsystems that were
	// previously inlined on Server (the "God Object"). Server now delegates
	// in-editor chat and code-completion behavior to these cohesive services,
	// each of which holds a back-reference to the Server for shared
	// infrastructure (config, LLM clients, stats, document access).
	chat           *chatService
	completion     *completionService
	configLoadOpts appconfig.LoadOptions
	// Outgoing JSON-RPC id counter for server-initiated requests
	nextID int64

	// Gitignore-aware file checker (nil when disabled)
	ignoreChecker *ignore.Checker

	// Dispatch table for JSON-RPC methods → handler functions
	handlers map[string]func(Request)
}

type codeActionSubsystem struct {
	llmClientRegistry
}

// llmStatsSubsystem holds atomic LLM request counters. All fields are
// lock-free (atomic.Int64), so no mutex is needed.
type llmStatsSubsystem struct {
	llmReqTotal       atomic.Int64
	llmSentBytesTotal atomic.Int64
	llmRespTotal      atomic.Int64
	llmRespBytesTotal atomic.Int64
	startTime         time.Time
}

// GlobalStatus bundles the fields for a global status update,
// replacing a long parameter list.
type GlobalStatus struct {
	Reqs      int64
	RPM       float64
	Sent      int64
	Recv      int64
	Provider  string
	Model     string
	ScopeRPM  float64
	ScopeReqs int64
	Window    time.Duration
}

// StatusSink receives status updates from the LSP server.
type StatusSink interface {
	// SetLLMStart records the provider and model for an LLM request that is starting.
	SetLLMStart(provider, model string) error
	// SetGlobal records aggregate LLM request and byte counters.
	SetGlobal(gs GlobalStatus) error
}

// ServerOptions collects configuration for NewServer to avoid long parameter lists.
type ServerOptions struct {
	LogContext        bool
	ConfigStore       *runtimeconfig.Store
	Config            *appconfig.App
	ConfigLoadOptions appconfig.LoadOptions

	Client llm.Client
	// Gitignore-aware file checker (optional)
	IgnoreChecker *ignore.Checker
	StatusSink    StatusSink
}

// NewServer creates a new LSP server that reads from r and writes to w.
func NewServer(r io.Reader, w io.Writer, logger *log.Logger, opts ServerOptions) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		in:           bufio.NewReader(r),
		out:          w,
		logger:       logger,
		docs:         make(map[string]*document),
		logContext:   opts.LogContext,
		configStore:  opts.ConfigStore,
		serverCtx:    ctx,
		serverCancel: cancel,
		codeActionSubsystem: codeActionSubsystem{
			llmClientRegistry: llmClientRegistry{},
		},
	}
	// Wire up the chat and completion services with a back-reference to the
	// server so they can reach shared infrastructure while owning their own
	// feature-specific state and logic.
	s.chat = newChatService(s)
	s.completion = newCompletionService(s)
	s.startTime = time.Now()
	s.applyOptions(opts)
	// Initialize dispatch table
	s.handlers = map[string]func(Request){
		"initialize":               s.handleInitialize,
		"initialized":              func(_ Request) { s.handleInitialized() },
		"shutdown":                 s.handleShutdown,
		"exit":                     func(_ Request) { s.handleExit() },
		"textDocument/didOpen":     s.handleDidOpen,
		"textDocument/didChange":   s.handleDidChange,
		"textDocument/didClose":    s.handleDidClose,
		"textDocument/completion":  s.completion.handleCompletion,
		"textDocument/codeAction":  s.handleCodeAction,
		"codeAction/resolve":       s.handleCodeActionResolve,
		"workspace/executeCommand": s.handleExecuteCommand,
	}
	return s
}

func (s *Server) applyOptions(opts ServerOptions) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logContext = opts.LogContext
	s.configLoadOpts = opts.ConfigLoadOptions
	if opts.ConfigStore != nil {
		s.configStore = opts.ConfigStore
	}
	if opts.Config != nil {
		s.cfg = *opts.Config
	} else if opts.ConfigStore != nil {
		s.cfg = opts.ConfigStore.Snapshot()
	} else {
		s.cfg = appconfig.App{}
	}
	if opts.IgnoreChecker != nil {
		s.ignoreChecker = opts.IgnoreChecker
	}
	if opts.StatusSink != nil {
		s.statusSink = opts.StatusSink
	}
	s.llmClientRegistry.applyOptions(opts.Client, s.cfg.Provider)
}

// ApplyOptions updates the server's configuration at runtime.
func (s *Server) ApplyOptions(opts ServerOptions) {
	s.applyOptions(opts)
}

func (s *Server) currentLLMClient() llm.Client {
	return s.llmClientRegistry.current()
}

func newClientForProvider(cfg appconfig.App, provider, modelOverride string) (llm.Client, error) {
	return llmutils.NewClientFromAppForProvider(cfg, provider, modelOverride)
}

func (s *Server) clientFor(spec requestSpec) llm.Client {
	return s.llmClientRegistry.clientFor(spec, s.currentConfig(), newClientForProvider)
}

func (s *Server) currentConfig() appconfig.App {
	s.mu.RLock()
	store := s.configStore
	cfg := s.cfg
	s.mu.RUnlock()
	if store != nil {
		return store.Snapshot()
	}
	return cfg
}

func (s *Server) maxTokens() int {
	cfg := s.currentConfig()
	if cfg.MaxTokens <= 0 {
		return 500
	}
	return cfg.MaxTokens
}

func (s *Server) contextMode() string {
	mode := strings.TrimSpace(s.currentConfig().ContextMode)
	if mode == "" {
		return "file-on-new-func"
	}
	return mode
}

func (s *Server) windowLines() int {
	cfg := s.currentConfig()
	if cfg.ContextWindowLines <= 0 {
		return 120
	}
	return cfg.ContextWindowLines
}

func (s *Server) maxContextTokens() int {
	cfg := s.currentConfig()
	if cfg.MaxContextTokens <= 0 {
		return 2000
	}
	return cfg.MaxContextTokens
}

func (s *Server) triggerCharacters() []string {
	cfg := s.currentConfig()
	if len(cfg.TriggerCharacters) == 0 {
		return []string{".", ":", "/", "_", ")", "{"}
	}
	return append([]string{}, cfg.TriggerCharacters...)
}

func (s *Server) codingTemperature() *float64 {
	cfg := s.currentConfig()
	return cfg.CodingTemperature
}

func (s *Server) manualInvokeMinPrefix() int {
	return s.currentConfig().ManualInvokeMinPrefix
}

func (s *Server) completionDebounce() time.Duration {
	cfg := s.currentConfig()
	if cfg.CompletionDebounceMs <= 0 {
		return 0
	}
	return time.Duration(cfg.CompletionDebounceMs) * time.Millisecond
}

func (s *Server) completionThrottle() time.Duration {
	cfg := s.currentConfig()
	if cfg.CompletionThrottleMs <= 0 {
		return 0
	}
	return time.Duration(cfg.CompletionThrottleMs) * time.Millisecond
}

func (s *Server) completionWaitAll() bool {
	cfg := s.currentConfig()
	if cfg.CompletionWaitAll == nil {
		return true // default: wait for all backends
	}
	return *cfg.CompletionWaitAll
}

// inlineMarkers returns the configured inline open/close marker strings along
// with their leading bytes. The close marker is named closeStr rather than
// close so it does not shadow the Go builtin close used for channels.
func (s *Server) inlineMarkers() (open string, closeStr string, openChar byte, closeChar byte) {
	cfg := s.currentConfig()
	open = strings.TrimSpace(cfg.InlineOpen)
	if open == "" {
		open = ">!"
	}
	closeStr = strings.TrimSpace(cfg.InlineClose)
	if closeStr == "" {
		closeStr = ">"
	}
	openChar = '>'
	if len(open) > 0 {
		openChar = open[0]
	}
	closeChar = '>'
	if len(closeStr) > 0 {
		closeChar = closeStr[0]
	}
	return open, closeStr, openChar, closeChar
}

func (s *Server) chatConfig() (suffix string, prefixes []string, suffixChar byte) {
	cfg := s.currentConfig()
	suffix = cfg.ChatSuffix
	if suffix != "" {
		suffix = strings.TrimSpace(suffix)
		if suffix == "" {
			suffix = ">"
		}
	} else {
		suffix = ""
	}
	if len(cfg.ChatPrefixes) == 0 {
		prefixes = []string{"?", "!", ":", ";"}
	} else {
		prefixes = append([]string{}, cfg.ChatPrefixes...)
	}
	suffixChar = '>'
	if len(suffix) > 0 {
		suffixChar = suffix[0]
	}
	return suffix, prefixes, suffixChar
}

func (s *Server) promptSet() appconfig.App {
	return s.currentConfig()
}

func (s *Server) customActions() []appconfig.CustomAction {
	cfg := s.currentConfig()
	if len(cfg.CustomActions) == 0 {
		return nil
	}
	return append([]appconfig.CustomAction{}, cfg.CustomActions...)
}

func (s *Server) requestTimeoutContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if s.serverCtx == nil {
		return context.WithTimeout(context.Background(), timeout)
	}
	return context.WithTimeout(s.serverCtx, timeout)
}

func (s *Server) cancelRequests() {
	if s.serverCancel != nil {
		s.serverCancel()
	}
}

// watchParentContext propagates cancellation from the caller-provided ctx into
// the server's own context. When ctx is cancelled we call cancelRequests so any
// in-flight LLM/network work tied to s.serverCtx is aborted; the main loop then
// returns once the current blocking read unblocks. It returns a stop function
// that tears down the watcher goroutine when Run exits normally (EOF), so the
// goroutine never leaks. A nil ctx (or context.Background) yields a no-op.
func (s *Server) watchParentContext(ctx context.Context) func() {
	if ctx == nil || ctx.Done() == nil {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			s.cancelRequests()
		case <-done:
		}
	}()
	return func() { close(done) }
}

func (s *Server) emitLLMStartStatus(provider, model string) {
	if s.statusSink != nil {
		if err := s.statusSink.SetLLMStart(provider, model); err != nil {
			logging.Logf("lsp ", "status sink SetLLMStart error: %v", err)
		}
	}
}

func (s *Server) emitGlobalStatus(gs GlobalStatus) {
	if s.statusSink != nil {
		if err := s.statusSink.SetGlobal(gs); err != nil {
			logging.Logf("lsp ", "status sink SetGlobal error: %v", err)
		}
	}
}

// Run starts the server's main loop, reading and dispatching LSP messages until EOF or exit.
// On shutdown it cancels the server context and waits for in-flight goroutines.
//
// The supplied ctx ties the serve loop to the process lifecycle: when the
// caller's context is cancelled (e.g. SIGINT/SIGTERM at main), we cancel the
// internal server context so in-flight LLM/network requests are aborted and the
// blocking stdin read is unblocked, letting Run return promptly.
func (s *Server) Run(ctx context.Context) error {
	stopWatch := s.watchParentContext(ctx)
	defer stopWatch()
	defer func() {
		s.cancelRequests()
		s.inflight.Wait()
	}()
	for {
		body, err := s.readMessage()
		if errors.Is(err, io.EOF) {
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
		// Track every request goroutine so Run's deferred inflight.Wait()
		// catches them all and prevents use-after-close writes to s.out.
		s.inflight.Add(1)
		go func(r Request) {
			defer s.inflight.Done()
			s.handle(r)
		}(req)
		if s.exited.Load() {
			return nil
		}
	}
}
