// Summary: Initialization and lifecycle handlers split from handlers.go.
package lsp

import (
	"codeberg.org/snonux/hexai/internal"
	"codeberg.org/snonux/hexai/internal/logging"
)

func (s *Server) handleInitialize(req Request) {
	client := s.currentLLMClient()
	version := internal.Version
	if client != nil {
		version = version + " [" + client.Name() + ":" + client.DefaultModel() + "]"
	}
	res := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: 1, // 1 = TextDocumentSyncKindFull
			CompletionProvider: &CompletionOptions{
				ResolveProvider:   false,
				TriggerCharacters: s.triggerCharacters(),
			},
			CodeActionProvider: CodeActionOptions{ResolveProvider: true},
		},
		ServerInfo: &ServerInfo{Name: "hexai", Version: version},
	}
	s.reply(req.ID, res, nil)
}

func (s *Server) handleInitialized() {
	logging.Logf("lsp ", "client initialized")
	// Emit an initial tmux heartbeat with provider/model
	if client := s.currentLLMClient(); client != nil {
		s.emitLLMStartStatus(client.Name(), client.DefaultModel())
	}
}

func (s *Server) handleShutdown(req Request) {
	s.cancelRequests()
	s.reply(req.ID, nil, nil)
}

func (s *Server) handleExit() {
	s.cancelRequests()
	s.exited.Store(true)
}
