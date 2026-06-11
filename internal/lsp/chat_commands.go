package lsp

import (
	"fmt"
	"strings"

	"codeberg.org/snonux/hexai/internal/runtimeconfig"
)

type chatCommandResult struct {
	message string
}

func (c *chatService) chatCommandResponse(uri string, lineIdx int, prompt string) (chatCommandResult, bool) {
	trimmed := strings.TrimSpace(c.stripTrailingTrigger(prompt))
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return chatCommandResult{}, false
	}

	switch {
	case strings.HasPrefix(trimmed, "/reload"):
		return c.handleReloadCommand(), true
	case strings.HasPrefix(trimmed, "/help"):
		return c.handleHelpCommand(), true
	case strings.HasPrefix(trimmed, "/disable"):
		return c.handleDisableCompletionCommand(), true
	case strings.HasPrefix(trimmed, "/enable"):
		return c.handleEnableCompletionCommand(), true
	default:
		return chatCommandResult{message: fmt.Sprintf("Unknown command %q. Try /help?>", trimmed)}, true
	}
}

func (c *chatService) handleHelpCommand() chatCommandResult {
	lines := []string{
		"Available slash commands:",
		"- /reload?> reload configuration from file (ignores env overrides)",
		"- /disable?> disable auto-completions for this session",
		"- /enable?> re-enable auto-completions",
	}
	return chatCommandResult{message: strings.Join(lines, "\n")}
}

func (c *chatService) handleReloadCommand() chatCommandResult {
	s := c.srv
	if s.configStore == nil {
		return chatCommandResult{message: "Reload unavailable: no config store"}
	}
	loadOpts := s.configLoadOpts
	loadOpts.IgnoreEnv = true
	// Tie the reload's blocking file reads to the server context so an
	// in-progress reload is abandoned if the server is shutting down.
	changes, err := s.configStore.Reload(s.serverCtx, s.logger, loadOpts)
	if err != nil {
		s.logger.Printf("config reload failed: %v", err)
		return chatCommandResult{message: fmt.Sprintf("Reload failed: %v", err)}
	}
	summary := runtimeconfig.FormatSummary("Reloaded config", changes)
	s.logger.Print(summary)
	return chatCommandResult{message: summary}
}

func (c *chatService) handleDisableCompletionCommand() chatCommandResult {
	prev := c.srv.setCompletionsDisabled(true)
	if prev {
		return chatCommandResult{message: "Auto-completions were already disabled."}
	}
	return chatCommandResult{message: "Auto-completions disabled. Use /enable?> to restore."}
}

func (c *chatService) handleEnableCompletionCommand() chatCommandResult {
	prev := c.srv.setCompletionsDisabled(false)
	if !prev {
		return chatCommandResult{message: "Auto-completions are already enabled."}
	}
	return chatCommandResult{message: "Auto-completions enabled."}
}
