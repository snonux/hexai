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
	in         *bufio.Reader
	out        io.Writer
	logger     *log.Logger
	exited     bool
	mu         sync.RWMutex
	docs       map[string]*document
	logContext bool
    llmClient  llm.Client
    lastInput  time.Time
    maxTokens  int
    contextMode      string
    windowLines      int
    maxContextTokens int
    noDiskIO         bool
}

func NewServer(r io.Reader, w io.Writer, logger *log.Logger, logContext bool, maxTokens int, contextMode string, windowLines int, maxContextTokens int, noDiskIO bool) *Server {
    s := &Server{in: bufio.NewReader(r), out: w, logger: logger, docs: make(map[string]*document), logContext: logContext}
    if maxTokens <= 0 {
        maxTokens = 500
    }
    s.maxTokens = maxTokens
    if contextMode == "" {
        contextMode = "file-on-new-func"
    }
    if windowLines <= 0 {
        windowLines = 120
    }
    if maxContextTokens <= 0 {
        maxContextTokens = 2000
    }
    s.contextMode = contextMode
    s.windowLines = windowLines
    s.maxContextTokens = maxContextTokens
    s.noDiskIO = noDiskIO
    if c, err := llm.NewDefault(); err != nil {
        logging.Logf("lsp ", "llm disabled: %v", err)
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
