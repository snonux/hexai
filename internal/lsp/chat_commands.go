package lsp

import (
	"fmt"
	"strings"

	"codeberg.org/snonux/hexai/internal/appconfig"
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
	changes, err := s.configStore.Reload(s.logger, appconfig.LoadOptions{IgnoreEnv: true})
	if err != nil {
		s.logger.Printf("config reload failed: %v", err)
		return chatCommandResult{message: fmt.Sprintf("Reload failed: %v", err)}
	}
	summary := formatReloadSummary(changes)
	s.logger.Print(summary)
	return chatCommandResult{message: summary}
}

func formatReloadSummary(changes []runtimeconfig.Change) string {
	if len(changes) == 0 {
		return "Reloaded config (no changes detected)."
	}
	lines := make([]string, 0, len(changes)+1)
	lines = append(lines, fmt.Sprintf("Reloaded config (%d changes):", len(changes)))
	for _, ch := range changes {
		lines = append(lines, fmt.Sprintf("- %s: %s → %s", ch.Key, ch.Old, ch.New))
	}
	return strings.Join(lines, "\n")
}
