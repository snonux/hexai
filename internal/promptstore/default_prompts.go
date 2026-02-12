// Summary: Built-in meta-prompts for prompt management.
package promptstore

import (
	"time"
)

// DefaultPrompts returns the built-in meta-prompts for prompt management.
// These prompts help users create, update, delete, and design prompts interactively
// using Claude's access to conversation context.
func DefaultPrompts() []Prompt {
	now := time.Now()

	return []Prompt{
		{
			Name:        "save_prompt",
			Title:       "Save Current Conversation as Prompt",
			Description: "Interactively create a new prompt template from the current conversation. Claude will analyze the conversation, ask clarifying questions about templating, show a preview, and wait for approval before saving.",
			Arguments: []PromptArgument{
				{
					Name:        "prompt_name",
					Description: "Unique identifier for the new prompt (lowercase, underscores allowed)",
					Required:    true,
				},
				{
					Name:        "prompt_title",
					Description: "Human-readable display name for the new prompt",
					Required:    true,
				},
			},
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: MessageContent{
						Type: "text",
						Text: `I want to create a new prompt template named '{{prompt_name}}' with title '{{prompt_title}}'.

Please help me by:
1) Analyzing our current conversation to understand what should be templated
2) Asking me clarifying questions about:
   - What parts should be template arguments vs fixed text
   - What description would best explain this prompt's purpose
   - What tags would help categorize it
   - Whether multi-turn messages are needed
3) Showing me a complete preview of the prompt structure in a code block
4) Only after I approve, use the create_prompt tool to save it

IMPORTANT FORMATTING RULES for clarifying questions:
- Use numbered questions: 1), 2), 3)
- ANY CHOICE MUST BE NUMBERED using combined format: 1a), 1b), 1c), 2a), 2b), etc.
- NEVER use standalone letters like "a)" - always combine with question number
- NEVER use dashes (-) or bullets (•) for options
- Every option must be numbered for easy selection by the user

Examples:
  1) Question Category
  Which do you prefer?
  1a) First option
  1b) Second option
  1c) Third option

  2) Arguments
  Should this accept parameters?
  2a) No arguments - fixed behavior
  2b) Optional file_pattern argument
  2c) Multiple optional arguments

  3) Sub-items
  Consider these aspects:
  3a) First aspect to consider
  3b) Second aspect to consider
  3c) Third aspect to consider

  4) Multiple sub-questions
  4a) Sub-question one?
      Answer options here
  4b) Sub-question two?
      Answer options here

Start by examining our conversation and asking your clarifying questions using this format.`,
					},
				},
			},
			Tags:    []string{"meta", "prompt-management", "interactive"},
			Created: now,
			Updated: now,
		},
		{
			Name:        "update_prompt",
			Title:       "Update Existing Prompt",
			Description: "Interactively modify an existing prompt. Claude will fetch the current version, ask what changes you want, show a preview with changes highlighted, and wait for approval before updating.",
			Arguments: []PromptArgument{
				{
					Name:        "prompt_name",
					Description: "Name of the existing prompt to update",
					Required:    true,
				},
			},
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: MessageContent{
						Type: "text",
						Text: `I want to update the existing prompt '{{prompt_name}}'.

Please help me by:
1) Ask me what changes I want to make to the prompt '{{prompt_name}}' (title, description, arguments, messages, or tags)
2) If I reference content from our current conversation, help extract and template it
3) Show me a complete preview of the updated prompt with changes highlighted
4) Only after I approve, use the update_prompt tool to save the changes

IMPORTANT FORMATTING RULES for clarifying questions:
- Use numbered questions: 1), 2), 3)
- ANY CHOICE MUST BE NUMBERED using combined format: 1a), 1b), 1c), 2a), 2b), etc.
- NEVER use standalone letters like "a)" - always combine with question number
- NEVER use dashes (-) or bullets (•) for options
- Every option must be numbered for easy selection by the user

Examples:
  1) Question Category
  Which do you prefer?
  1a) First option
  1b) Second option
  1c) Third option

  2) Changes to Make
  What aspects should be updated?
  2a) Description only
  2b) Arguments only
  2c) Both description and arguments
  2d) Complete rewrite

  3) Multiple aspects
  Consider:
  3a) First aspect to evaluate
  3b) Second aspect to evaluate
  3c) Third aspect to evaluate

Start by asking me what changes I want to make, using this format for any clarifying questions.`,
					},
				},
			},
			Tags:    []string{"meta", "prompt-management", "interactive"},
			Created: now,
			Updated: now,
		},
		{
			Name:        "delete_prompt",
			Title:       "Delete Custom Prompt",
			Description: "Interactively delete an existing custom prompt with confirmation. Claude will show the current prompt, ask for confirmation, and only delete after explicit approval. Built-in prompts cannot be deleted.",
			Arguments: []PromptArgument{
				{
					Name:        "prompt_name",
					Description: "Name of the existing prompt to delete",
					Required:    true,
				},
			},
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: MessageContent{
						Type: "text",
						Text: `I want to delete the existing prompt '{{prompt_name}}'.

Please help me by:
1) Confirm with me that I want to delete the prompt named '{{prompt_name}}'
2) Explain that this action cannot be undone (though backups are automatically created)
3) Ask me to type 'yes' to confirm the deletion
4) Only after I explicitly confirm with 'yes', use the delete_prompt tool to delete it

IMPORTANT NOTES:
- Built-in prompts (save_prompt, update_prompt, delete_prompt) cannot be deleted
- Only custom prompts stored in user.jsonl can be deleted
- Backups are automatically created before deletion

Ask me to confirm the deletion of '{{prompt_name}}'.`,
					},
				},
			},
			Tags:    []string{"meta", "prompt-management", "interactive"},
			Created: now,
			Updated: now,
		},
		{
			Name:        "design_prompt",
			Title:       "Design New Prompt from Scratch",
			Description: "Interactively design a brand new prompt template through guided questions. Claude will help you define the purpose, arguments, message flow, and metadata step by step, show a preview, and wait for approval before saving.",
			Arguments:   []PromptArgument{},
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: MessageContent{
						Type: "text",
						Text: `I want to design a brand new prompt template from scratch.

Please ask me:
1) What should this prompt do? (describe the task/purpose in 1-2 sentences)
2) What arguments does it need? (if any - use {{argument}} syntax)
3) Prompt name (lowercase, underscores only), title, and tags

Then show me a preview and save it after I approve.

Keep questions brief and focused.`,
					},
				},
			},
			Tags:    []string{"meta", "prompt-management", "interactive", "creation"},
			Created: now,
			Updated: now,
		},
	}
}
