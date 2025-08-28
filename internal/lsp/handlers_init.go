// Summary: Initialization and lifecycle handlers split from handlers.go.
package lsp

import (
	"hexai/internal"
	"hexai/internal/logging"
	"os"
)

func (s *Server) handleInitialize(req Request) {
	version := internal.Version
	if s.llmClient != nil {
		version = version + " [" + s.llmClient.Name() + ":" + s.llmClient.DefaultModel() + "]"
	}
	res := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: 1, // 1 = TextDocumentSyncKindFull
			CompletionProvider: &CompletionOptions{
				ResolveProvider:   false,
				TriggerCharacters: s.triggerChars,
			},
			CodeActionProvider: CodeActionOptions{ResolveProvider: true},
		},
		ServerInfo: &ServerInfo{Name: "hexai", Version: version},
	}
	s.reply(req.ID, res, nil)
}

func (s *Server) handleInitialized() {
	logging.Logf("lsp ", "client initialized")
}

func (s *Server) handleShutdown(req Request) {
	s.reply(req.ID, nil, nil)
}

func (s *Server) handleExit() {
	s.exited = true
	os.Exit(0)
}
