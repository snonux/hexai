// Summary: Built-in prompts for common development tasks.
package promptstore

import "time"

// GetBuiltinPrompts returns the default set of prompts.
// These are written to default.jsonl on first run.
func GetBuiltinPrompts() []Prompt {
	now := time.Now()

	return []Prompt{
		{
			Name:        "code_review",
			Title:       "Request Code Review",
			Description: "Analyzes code quality, style, and suggests improvements",
			Arguments: []PromptArgument{
				{Name: "code", Description: "The code to review", Required: true},
			},
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: MessageContent{
						Type: "text",
						Text: "Please review the following code for quality, style, and potential issues:\n\n{{code}}",
					},
				},
			},
			Tags:    []string{"development", "review", "quality"},
			Created: now,
			Updated: now,
		},
		{
			Name:        "explain_code",
			Title:       "Explain Code",
			Description: "Provides detailed explanation of what code does",
			Arguments: []PromptArgument{
				{Name: "code", Description: "The code to explain", Required: true},
			},
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: MessageContent{
						Type: "text",
						Text: "Please explain in detail what the following code does:\n\n{{code}}",
					},
				},
			},
			Tags:    []string{"development", "documentation", "learning"},
			Created: now,
			Updated: now,
		},
		{
			Name:        "generate_tests",
			Title:       "Generate Unit Tests",
			Description: "Generates unit tests for a function or class",
			Arguments: []PromptArgument{
				{Name: "code", Description: "The code to test", Required: true},
				{Name: "language", Description: "Programming language", Required: false},
			},
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: MessageContent{
						Type: "text",
						Text: "Generate comprehensive unit tests for the following code:\n\nLanguage: {{language}}\n\n{{code}}",
					},
				},
			},
			Tags:    []string{"development", "testing", "tdd"},
			Created: now,
			Updated: now,
		},
		{
			Name:        "document_function",
			Title:       "Generate Documentation",
			Description: "Generates documentation comments and docstrings",
			Arguments: []PromptArgument{
				{Name: "code", Description: "The code to document", Required: true},
			},
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: MessageContent{
						Type: "text",
						Text: "Generate comprehensive documentation for the following code:\n\n{{code}}",
					},
				},
			},
			Tags:    []string{"development", "documentation"},
			Created: now,
			Updated: now,
		},
		{
			Name:        "simplify_code",
			Title:       "Simplify Code",
			Description: "Simplifies complex code while preserving behavior",
			Arguments: []PromptArgument{
				{Name: "code", Description: "The code to simplify", Required: true},
			},
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: MessageContent{
						Type: "text",
						Text: "Simplify the following code while preserving its behavior:\n\n{{code}}",
					},
				},
			},
			Tags:    []string{"development", "refactoring", "quality"},
			Created: now,
			Updated: now,
		},
		{
			Name:        "fix_bugs",
			Title:       "Analyze and Fix Bugs",
			Description: "Analyzes code for bugs and suggests fixes",
			Arguments: []PromptArgument{
				{Name: "code", Description: "The code to analyze", Required: true},
				{Name: "error", Description: "Error message or symptoms", Required: false},
			},
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: MessageContent{
						Type: "text",
						Text: "Analyze the following code for bugs and suggest fixes:\n\nError: {{error}}\n\nCode:\n{{code}}",
					},
				},
			},
			Tags:    []string{"development", "debugging", "bug-fix"},
			Created: now,
			Updated: now,
		},
		{
			Name:        "refactor_extract",
			Title:       "Extract Function",
			Description: "Extracts code into a separate, reusable function",
			Arguments: []PromptArgument{
				{Name: "code", Description: "The code to extract", Required: true},
				{Name: "function_name", Description: "Desired function name", Required: false},
			},
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: MessageContent{
						Type: "text",
						Text: "Extract the following code into a separate function named {{function_name}}:\n\n{{code}}",
					},
				},
			},
			Tags:    []string{"development", "refactoring"},
			Created: now,
			Updated: now,
		},
	}
}
