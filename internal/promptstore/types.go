// Data models for prompt storage (templates with arguments).
package promptstore

import "time"

// Prompt represents a reusable prompt template with arguments.
// Prompts are stored in JSONL format and can be rendered with user-provided arguments.
type Prompt struct {
	Name        string           `json:"name"`        // Unique identifier (alphanumeric + underscores)
	Title       string           `json:"title"`       // Display name
	Description string           `json:"description"` // Human-readable description
	Arguments   []PromptArgument `json:"arguments"`   // Template variables
	Messages    []PromptMessage  `json:"messages"`    // Conversation messages
	Tags        []string         `json:"tags"`        // Categorization tags
	Created     time.Time        `json:"created"`     // Creation timestamp
	Updated     time.Time        `json:"updated"`     // Last update timestamp
}

// PromptArgument defines a template variable that can be substituted in messages.
// Used for parameterized prompts like "review {{code}}" where {{code}} is an argument.
type PromptArgument struct {
	Name        string `json:"name"`        // Variable name (used in {{name}})
	Description string `json:"description"` // Human-readable description
	Required    bool   `json:"required"`    // Whether argument is required
}

// PromptMessage represents a single message in a conversation.
// Messages can be from user or assistant roles and contain text content.
type PromptMessage struct {
	Role    string         `json:"role"`    // "user" or "assistant"
	Content MessageContent `json:"content"` // Message content
}

// MessageContent contains the actual message data.
// Currently supports text content; extensible for images/resources in future.
type MessageContent struct {
	Type string `json:"type"` // "text", "image", "resource"
	Text string `json:"text,omitempty"`
}
