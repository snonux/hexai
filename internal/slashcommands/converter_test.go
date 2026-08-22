package slashcommands

import (
	"strings"
	"testing"

	"github.com/snonux/hexai/internal/promptstore"
)

func TestConvertPromptToMarkdown_MinimalPrompt(t *testing.T) {
	prompt := &promptstore.Prompt{
		Name:  "minimal",
		Title: "Minimal Prompt",
	}

	result := ConvertPromptToMarkdown(prompt)

	if !strings.Contains(result, "# Minimal Prompt") {
		t.Error("Markdown should contain title")
	}
	if !strings.Contains(result, "*Generated from MCP prompt: minimal*") {
		t.Error("Markdown should contain footer with prompt name")
	}
}

func TestConvertPromptToMarkdown_WithDescription(t *testing.T) {
	prompt := &promptstore.Prompt{
		Name:        "test",
		Title:       "Test Prompt",
		Description: "This is a test prompt description.",
	}

	result := ConvertPromptToMarkdown(prompt)

	if !strings.Contains(result, "This is a test prompt description.") {
		t.Error("Markdown should contain description")
	}
}

func TestConvertPromptToMarkdown_WithArguments(t *testing.T) {
	prompt := &promptstore.Prompt{
		Name:  "args-test",
		Title: "Arguments Test",
		Arguments: []promptstore.PromptArgument{
			{Name: "required_arg", Description: "A required argument", Required: true},
			{Name: "optional_arg", Description: "An optional argument", Required: false},
		},
	}

	result := ConvertPromptToMarkdown(prompt)

	if !strings.Contains(result, "## Usage") {
		t.Error("Markdown should contain Usage section")
	}
	if !strings.Contains(result, "**required_arg** (required)") {
		t.Error("Markdown should mark required arguments")
	}
	if !strings.Contains(result, "**optional_arg** (optional)") {
		t.Error("Markdown should mark optional arguments")
	}
	if !strings.Contains(result, "A required argument") {
		t.Error("Markdown should contain argument descriptions")
	}
}

func TestConvertPromptToMarkdown_WithMessages(t *testing.T) {
	prompt := &promptstore.Prompt{
		Name:  "msg-test",
		Title: "Messages Test",
		Messages: []promptstore.PromptMessage{
			{
				Role: "user",
				Content: promptstore.MessageContent{
					Type: "text",
					Text: "This is the template with {{placeholder}}",
				},
			},
			{
				Role: "assistant",
				Content: promptstore.MessageContent{
					Type: "text",
					Text: "This should be ignored",
				},
			},
		},
	}

	result := ConvertPromptToMarkdown(prompt)

	if !strings.Contains(result, "## Template") {
		t.Error("Markdown should contain Template section")
	}
	if !strings.Contains(result, "This is the template with {{placeholder}}") {
		t.Error("Markdown should contain user message content")
	}
	if strings.Contains(result, "This should be ignored") {
		t.Error("Markdown should only include first user message in template")
	}
}

func TestConvertPromptToMarkdown_WithTags(t *testing.T) {
	prompt := &promptstore.Prompt{
		Name:  "tags-test",
		Title: "Tags Test",
		Tags:  []string{"coding", "review", "refactor"},
	}

	result := ConvertPromptToMarkdown(prompt)

	if !strings.Contains(result, "## Tags") {
		t.Error("Markdown should contain Tags section")
	}
	if !strings.Contains(result, "coding, review, refactor") {
		t.Error("Markdown should contain all tags as comma-separated list")
	}
}

func TestConvertPromptToMarkdown_FullPrompt(t *testing.T) {
	prompt := &promptstore.Prompt{
		Name:        "full-example",
		Title:       "Full Example Prompt",
		Description: "A complete example with all fields.",
		Arguments: []promptstore.PromptArgument{
			{Name: "input", Description: "Input text", Required: true},
			{Name: "format", Description: "Output format", Required: false},
		},
		Messages: []promptstore.PromptMessage{
			{
				Role: "user",
				Content: promptstore.MessageContent{
					Type: "text",
					Text: "Process {{input}} with format {{format}}",
				},
			},
		},
		Tags: []string{"example", "test"},
	}

	result := ConvertPromptToMarkdown(prompt)

	// Verify all sections are present in order
	sections := []string{
		"# Full Example Prompt",
		"A complete example with all fields.",
		"## Usage",
		"**input** (required)",
		"**format** (optional)",
		"## Template",
		"Process {{input}} with format {{format}}",
		"## Tags",
		"example, test",
		"---",
		"*Generated from MCP prompt: full-example*",
	}

	for _, section := range sections {
		if !strings.Contains(result, section) {
			t.Errorf("Markdown missing section: %q", section)
		}
	}
}

func TestFormatArguments(t *testing.T) {
	args := []promptstore.PromptArgument{
		{Name: "arg1", Description: "First arg", Required: true},
		{Name: "arg2", Description: "Second arg", Required: false},
	}

	result := formatArguments(args)

	if !strings.Contains(result, "## Usage") {
		t.Error("formatArguments should include Usage header")
	}
	if !strings.Contains(result, "**arg1** (required): First arg") {
		t.Error("formatArguments should format required arguments correctly")
	}
	if !strings.Contains(result, "**arg2** (optional): Second arg") {
		t.Error("formatArguments should format optional arguments correctly")
	}
}

func TestFormatMessages_FirstUserMessage(t *testing.T) {
	messages := []promptstore.PromptMessage{
		{Role: "assistant", Content: promptstore.MessageContent{Text: "Should skip"}},
		{Role: "user", Content: promptstore.MessageContent{Text: "First user message"}},
		{Role: "user", Content: promptstore.MessageContent{Text: "Second user message"}},
	}

	result := formatMessages(messages)

	if result != "First user message" {
		t.Errorf("formatMessages() = %q, want %q", result, "First user message")
	}
}

func TestFormatMessages_NoUserMessage(t *testing.T) {
	messages := []promptstore.PromptMessage{
		{Role: "assistant", Content: promptstore.MessageContent{Text: "Only assistant"}},
	}

	result := formatMessages(messages)

	if result != "(No template content)" {
		t.Errorf("formatMessages() should return placeholder when no user message, got %q", result)
	}
}

func TestFormatMessages_EmptyMessages(t *testing.T) {
	messages := []promptstore.PromptMessage{}

	result := formatMessages(messages)

	if result != "(No template content)" {
		t.Errorf("formatMessages() should return placeholder for empty messages, got %q", result)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple-name", "simple-name"},
		{"Name With Spaces", "name-with-spaces"},
		{"special!@#$chars", "special-chars"},
		{"Multiple___Underscores", "multiple___underscores"},
		{"dots.and.dashes", "dots-and-dashes"},
		{"--leading-trailing--", "leading-trailing"},
		{"UPPERCASE", "uppercase"},
		{"mixed_Case-123", "mixed_case-123"},
		{"unicode-café", "unicode-caf"},
	}

	for _, tt := range tests {
		result := sanitizeFilename(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMakeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "hexai-simple.md"},
		{"with spaces", "hexai-with-spaces.md"},
		{"Special_123", "hexai-special_123.md"},
		{"--trim--", "hexai-trim.md"},
	}

	for _, tt := range tests {
		result := MakeFilename(tt.input)
		if result != tt.expected {
			t.Errorf("MakeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMakeFilename_AlwaysHasPrefix(t *testing.T) {
	result := MakeFilename("test")
	if !strings.HasPrefix(result, "hexai-") {
		t.Error("MakeFilename should always add hexai- prefix")
	}
}

func TestMakeFilename_AlwaysHasExtension(t *testing.T) {
	result := MakeFilename("test")
	if !strings.HasSuffix(result, ".md") {
		t.Error("MakeFilename should always add .md extension")
	}
}
