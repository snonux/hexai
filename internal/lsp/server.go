// Summary: Minimal LSP server over stdio; manages documents, dispatches requests, and tracks stats.
package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/ignore"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/llmutils"
	"codeberg.org/snonux/hexai/internal/logging"
	"codeberg.org/snonux/hexai/internal/runtimeconfig"
)

// Server implements a minimal LSP over stdio.
type Server struct {
	in           *bufio.Reader
	out          io.Writer
	outMu        sync.Mutex
	logger       *log.Logger
	serverCtx    context.Context
	serverCancel context.CancelFunc
	exited       atomic.Bool
	mu           sync.RWMutex
	docs         map[string]*document
	logContext   bool
	configStore  *runtimeconfig.Store
	cfg          appconfig.App
	llmClient    llm.Client
	codeActionSubsystem
	chatSubsystem
	// LLM request stats
	llmReqTotal       int64
	llmSentBytesTotal int64
	llmRespTotal      int64
	llmRespBytesTotal int64
	startTime         time.Time
	completionSubsystem
	configLoadOpts appconfig.LoadOptions
	// Outgoing JSON-RPC id counter for server-initiated requests
	nextID int64

	// Gitignore-aware file checker (nil when disabled)
	ignoreChecker *ignore.Checker

	// Dispatch table for JSON-RPC methods → handler functions
	handlers map[string]func(Request)
}

type completionSubsystem struct {
	// Small LRU cache for recent code completion outputs (keyed by context)
	compCache           map[string]string
	compCacheOrder      []string // most-recent at end; cap ~10
	pendingCompletions  map[string][]CompletionItem
	lastLLMCall         time.Time
	completionsDisabled bool
}

type chatSubsystem struct {
	lastInput time.Time
}

type codeActionSubsystem struct {
	llmProvider string
	altClients  map[string]llm.Client
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
}

// CustomAction mirrors user-defined code actions passed from config.
type CustomAction struct {
	ID          string
	Title       string
	Kind        string
	Scope       string // "selection" | "diagnostics"
	Instruction string // if set, use rewrite templates
	System      string // optional when User is set
	User        string // if set, use this user template
}

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
	}
	s.startTime = time.Now()
	s.compCache = make(map[string]string)
	s.pendingCompletions = make(map[string][]CompletionItem)
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
		"textDocument/completion":  s.handleCompletion,
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
	s.llmClient = opts.Client
	if opts.Client != nil {
		s.llmProvider = canonicalProvider(opts.Client.Name())
	} else {
		s.llmProvider = canonicalProvider(s.cfg.Provider)
	}
	s.altClients = make(map[string]llm.Client)
	if opts.IgnoreChecker != nil {
		s.ignoreChecker = opts.IgnoreChecker
	}
}

// ApplyOptions updates the server's configuration at runtime.
func (s *Server) ApplyOptions(opts ServerOptions) {
	s.applyOptions(opts)
}

func (s *Server) currentLLMClient() llm.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.llmClient
}

func newClientForProvider(cfg appconfig.App, provider, modelOverride string) (llm.Client, error) {
	return llmutils.NewClientFromAppForProvider(cfg, provider, modelOverride)
}

func (s *Server) clientFor(spec requestSpec) llm.Client {
	provider := canonicalProvider(spec.provider)
	s.mu.RLock()
	baseProvider := s.llmProvider
	baseClient := s.llmClient
	if baseClient != nil && strings.TrimSpace(baseProvider) == "" {
		baseProvider = canonicalProvider(baseClient.Name())
	}
	if provider == "" {
		provider = baseProvider
	}
	if provider == baseProvider && baseClient != nil {
		s.mu.RUnlock()
		return baseClient
	}
	if c, ok := s.altClients[provider]; ok {
		s.mu.RUnlock()
		return c
	}
	cfg := s.cfg
	store := s.configStore
	s.mu.RUnlock()
	if store != nil {
		cfg = store.Snapshot()
	}
	modelOverride := strings.TrimSpace(spec.entry.Model)
	if modelOverride == "" {
		modelOverride = strings.TrimSpace(spec.fallbackModel)
	}
	client, err := newClientForProvider(cfg, provider, modelOverride)
	if err != nil {
		logging.Logf("lsp ", "failed to build client for provider=%s: %v", provider, err)
		if baseClient != nil {
			return baseClient
		}
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if provider == s.llmProvider {
		if s.llmClient == nil {
			s.llmClient = client
			s.llmProvider = provider
		}
		return s.llmClient
	}
	if existing, ok := s.altClients[provider]; ok {
		return existing
	}
	if s.altClients == nil {
		s.altClients = make(map[string]llm.Client)
	}
	s.altClients[provider] = client
	return client
}

func (s *Server) currentConfig() appconfig.App {
	if s.configStore != nil {
		return s.configStore.Snapshot()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *Server) storePendingCompletion(key string, items []CompletionItem) {
	if len(items) == 0 {
		return
	}
	cpy := make([]CompletionItem, len(items))
	copy(cpy, items)
	s.mu.Lock()
	if s.pendingCompletions == nil {
		s.pendingCompletions = make(map[string][]CompletionItem)
	}
	s.pendingCompletions[key] = cpy
	s.mu.Unlock()
}

func (s *Server) setCompletionsDisabled(disabled bool) bool {
	s.mu.Lock()
	prev := s.completionsDisabled
	s.completionsDisabled = disabled
	s.mu.Unlock()
	return prev
}

func (s *Server) completionDisabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.completionsDisabled
}

func (s *Server) takePendingCompletion(key string) []CompletionItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pendingCompletions) == 0 {
		return nil
	}
	items, ok := s.pendingCompletions[key]
	if !ok {
		return nil
	}
	delete(s.pendingCompletions, key)
	cpy := make([]CompletionItem, len(items))
	copy(cpy, items)
	return cpy
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

func (s *Server) inlineMarkers() (open string, close string, openChar byte, closeChar byte) {
	cfg := s.currentConfig()
	open = strings.TrimSpace(cfg.InlineOpen)
	if open == "" {
		open = ">!"
	}
	close = strings.TrimSpace(cfg.InlineClose)
	if close == "" {
		close = ">"
	}
	openChar = '>'
	if len(open) > 0 {
		openChar = open[0]
	}
	closeChar = '>'
	if len(close) > 0 {
		closeChar = close[0]
	}
	return open, close, openChar, closeChar
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

func (s *Server) customActions() []CustomAction {
	cfg := s.currentConfig()
	if len(cfg.CustomActions) == 0 {
		return nil
	}
	customs := make([]CustomAction, 0, len(cfg.CustomActions))
	for _, ca := range cfg.CustomActions {
		customs = append(customs, CustomAction{
			ID:          ca.ID,
			Title:       ca.Title,
			Kind:        ca.Kind,
			Scope:       ca.Scope,
			Instruction: ca.Instruction,
			System:      ca.System,
			User:        ca.User,
		})
	}
	return customs
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

func (s *Server) Run() error {
	defer s.cancelRequests()
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
		if s.exited.Load() {
			return nil
		}
	}
}
