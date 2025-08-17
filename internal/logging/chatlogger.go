package logging

// ChatLogger provides a structured way to log chat interactions.
type ChatLogger struct {
	Provider string
}

// NewChatLogger creates a new ChatLogger for a given provider.
func NewChatLogger(provider string) *ChatLogger {
	return &ChatLogger{Provider: provider}
}

// LogStart logs the beginning of a chat or stream interaction.
func (cl *ChatLogger) LogStart(stream bool, model string, temp float64, maxTokens int, stop []string, messages []struct {
	Role    string
	Content string
}) {
	chatOrStream := "chat"
	if stream {
		chatOrStream = "stream"
	}
	Logf("llm/"+cl.Provider+" ", "%s start model=%s temp=%.2f max_tokens=%d stop=%d messages=%d",
		chatOrStream, model, temp, maxTokens, len(stop), len(messages))
	for i, m := range messages {
		Logf("llm/"+cl.Provider+" ", "msg[%d] role=%s size=%d preview=%s%s%s",
			i, m.Role, len(m.Content), AnsiCyan, PreviewForLog(m.Content), AnsiBase)
	}
}
