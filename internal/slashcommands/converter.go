// Summary: Converts MCP prompts to generic slash command Markdown format.
package slashcommands

import (
	"fmt"
	"regexp"
	"strings"

	"codeberg.org/snonux/hexai/internal/promptstore"
)

// ConvertPromptToMarkdown converts an MCP prompt to slash command Markdown.
// Returns a formatted Markdown string suitable for writing to a .md file.
func ConvertPromptToMarkdown(prompt *promptstore.Prompt) string {
	var sb strings.Builder

	// Title
	sb.WriteString("# ")
	sb.WriteString(prompt.Title)
	sb.WriteString("\n\n")

	// Description
	if prompt.Description != "" {
		sb.WriteString(prompt.Description)
		sb.WriteString("\n\n")
	}

	// Arguments section
	if len(prompt.Arguments) > 0 {
		sb.WriteString(formatArguments(prompt.Arguments))
		sb.WriteString("\n")
	}

	// Template section (first user message)
	if len(prompt.Messages) > 0 {
		sb.WriteString("## Template\n\n")
		sb.WriteString(formatMessages(prompt.Messages))
		sb.WriteString("\n\n")
	}

	// Tags section
	if len(prompt.Tags) > 0 {
		sb.WriteString("## Tags\n\n")
		sb.WriteString(strings.Join(prompt.Tags, ", "))
		sb.WriteString("\n\n")
	}

	// Footer
	sb.WriteString("---\n")
	sb.WriteString("*Generated from MCP prompt: ")
	sb.WriteString(prompt.Name)
	sb.WriteString("*\n")

	return sb.String()
}

// formatArguments creates the Usage section with argument documentation.
// Returns formatted Markdown listing all arguments with their properties.
func formatArguments(args []promptstore.PromptArgument) string {
	var sb strings.Builder

	sb.WriteString("## Usage\n\n")
	sb.WriteString("This prompt template accepts the following arguments:\n\n")

	for _, arg := range args {
		// Format: - **arg_name** (required/optional): Description
		sb.WriteString("- **")
		sb.WriteString(arg.Name)
		sb.WriteString("** (")
		if arg.Required {
			sb.WriteString("required")
		} else {
			sb.WriteString("optional")
		}
		sb.WriteString("): ")
		sb.WriteString(arg.Description)
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatMessages extracts the first user message for the Template section.
// Shows users what the prompt template looks like with {{placeholders}}.
func formatMessages(messages []promptstore.PromptMessage) string {
	// Find first user message
	for _, msg := range messages {
		if msg.Role == "user" && msg.Content.Text != "" {
			return msg.Content.Text
		}
	}
	return "(No template content)"
}

// sanitizeFilename ensures prompt name is valid for filename.
// Replaces non-alphanumeric characters with hyphens.
func sanitizeFilename(name string) string {
	// Replace any character that's not alphanumeric or underscore with hyphen
	re := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	sanitized := re.ReplaceAllString(name, "-")

	// Remove leading/trailing hyphens
	sanitized = strings.Trim(sanitized, "-")

	// Convert to lowercase for consistency
	return strings.ToLower(sanitized)
}

// MakeFilename creates a slash command filename with hexai- prefix.
// Returns: hexai-{sanitized_name}.md
func MakeFilename(promptName string) string {
	return fmt.Sprintf("hexai-%s.md", sanitizeFilename(promptName))
}
