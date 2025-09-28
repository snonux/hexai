package lsp

import (
	"fmt"
	"strings"

	"codeberg.org/snonux/hexai/internal/runtimeconfig"
)

type chatCommandResult struct {
	message string
}

func (s *Server) chatCommandResponse(uri string, lineIdx int, prompt string) (chatCommandResult, bool) {
	trimmed := strings.TrimSpace(s.stripTrailingTrigger(prompt))
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return chatCommandResult{}, false
	}

	switch {
	case strings.HasPrefix(trimmed, "/reload"):
		return s.handleReloadCommand(), true
	case strings.HasPrefix(trimmed, "/help"):
		return s.handleHelpCommand(), true
	default:
		return chatCommandResult{message: fmt.Sprintf("Unknown command %q. Try /help?>", trimmed)}, true
	}
}

func (s *Server) handleHelpCommand() chatCommandResult {
	lines := []string{
		"Available slash commands:",
		"- /reload?> reload configuration from file (ignores env overrides)",
	}
	return chatCommandResult{message: strings.Join(lines, "\n")}
}

func (s *Server) handleReloadCommand() chatCommandResult {
	if s.configStore == nil {
		return chatCommandResult{message: "Reload unavailable: no config store"}
	}
	loadOpts := s.configLoadOpts
	loadOpts.IgnoreEnv = true
	changes, err := s.configStore.Reload(s.logger, loadOpts)
	if err != nil {
		s.logger.Printf("config reload failed: %v", err)
		return chatCommandResult{message: fmt.Sprintf("Reload failed: %v", err)}
	}
	summary := runtimeconfig.FormatSummary("Reloaded config", changes)
	s.logger.Print(summary)
	return chatCommandResult{message: summary}
}
