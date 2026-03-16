// Summary: MCP tool-related handler methods for listing tools, calling tools,
// and JSON Schema definitions for tool inputs.
package mcp

import (
	"encoding/json"
	"fmt"
)

// handleToolsList processes the tools/list request.
// Returns available tools for prompt management operations.
func (s *Server) handleToolsList(req Request) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		s.sendError(req.ID, ErrCodeInvalidRequest, "Server not initialized")
		return
	}
	s.mu.RUnlock()

	// Define 3 tools for prompt management
	tools := []Tool{
		s.makeCreatePromptTool(),
		s.makeUpdatePromptTool(),
		s.makeDeletePromptTool(),
	}

	result := ListToolsResult{
		Tools:      tools,
		NextCursor: "", // No pagination needed for 3 tools
	}

	s.sendResponse(req.ID, result)
}

// makeCreatePromptTool returns the JSON Schema definition for create_prompt tool.
func (s *Server) makeCreatePromptTool() Tool {
	return Tool{
		Name:        "create_prompt",
		Description: "Creates a new custom prompt template that can be invoked via the MCP prompts capability. The prompt is saved to user.jsonl and becomes immediately available for use.",
		InputSchema: s.buildCreatePromptSchema(),
	}
}

// buildCreatePromptSchema constructs the JSON Schema for create_prompt tool inputs.
func (s *Server) buildCreatePromptSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Unique identifier for the prompt (lowercase, underscores allowed)",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Human-readable display name for the prompt",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Detailed explanation of what the prompt does",
			},
			"arguments": buildArgumentArraySchema(),
			"messages":  buildMessageArraySchema(),
			"tags": map[string]interface{}{
				"type":        "array",
				"description": "Optional tags for categorizing the prompt",
				"items":       map[string]interface{}{"type": "string"},
			},
		},
		"required": []string{"name", "title", "messages"},
	}
}

// makeUpdatePromptTool returns the JSON Schema definition for update_prompt tool.
func (s *Server) makeUpdatePromptTool() Tool {
	return Tool{
		Name:        "update_prompt",
		Description: "Updates an existing custom prompt template. Only custom prompts (stored in user.jsonl) can be updated; built-in prompts cannot be modified.",
		InputSchema: s.buildUpdatePromptSchema(),
	}
}

// buildUpdatePromptSchema constructs the JSON Schema for update_prompt tool inputs.
func (s *Server) buildUpdatePromptSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the existing prompt to update",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "New display name (optional, only if changing)",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "New description (optional, only if changing)",
			},
			"arguments": buildArgumentArraySchema(),
			"messages":  buildMessageArraySchema(),
			"tags": map[string]interface{}{
				"type":        "array",
				"description": "New tags array (optional, replaces existing if provided)",
				"items":       map[string]interface{}{"type": "string"},
			},
		},
		"required": []string{"name"},
	}
}

// makeDeletePromptTool returns the JSON Schema definition for delete_prompt tool.
func (s *Server) makeDeletePromptTool() Tool {
	return Tool{
		Name:        "delete_prompt",
		Description: "Deletes a custom prompt template. Only custom prompts (stored in user.jsonl) can be deleted; built-in prompts cannot be removed.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the prompt to delete",
				},
			},
			"required": []string{"name"},
		},
	}
}

// handleToolsCall processes the tools/call request.
// Dispatches to appropriate tool wrapper based on tool name.
func (s *Server) handleToolsCall(req Request) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		s.sendError(req.ID, ErrCodeInvalidRequest, "Server not initialized")
		return
	}
	s.mu.RUnlock()

	var params CallToolRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid tools/call params")
		return
	}

	// Dispatch to appropriate tool wrapper
	switch params.Name {
	case "create_prompt":
		s.callCreatePromptTool(req.ID, params.Arguments)
	case "update_prompt":
		s.callUpdatePromptTool(req.ID, params.Arguments)
	case "delete_prompt":
		s.callDeletePromptTool(req.ID, params.Arguments)
	default:
		s.sendToolError(req.ID, fmt.Sprintf("Unknown tool: %s", params.Name))
	}
}

// callCreatePromptTool executes the create_prompt tool.
// Converts map arguments to CreatePromptRequest and delegates to store.Create.
func (s *Server) callCreatePromptTool(id any, args map[string]interface{}) {
	// Convert map to CreatePromptRequest
	params, err := convertToCreatePromptRequest(args)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Invalid arguments: %v", err))
		return
	}

	// Validate required fields
	if err := validateCreateParams(params); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	// Build prompt and create
	prompt := buildPromptFromCreateParams(params)
	if err := s.store.Create(prompt); err != nil {
		s.logger.Printf("create prompt error: %v", err)
		s.sendToolError(id, fmt.Sprintf("Failed to create prompt: %v", err))
		return
	}

	s.logger.Printf("created prompt via tool: %s", params.Name)

	// Sync to slash commands if enabled
	if s.syncer != nil {
		if err := s.syncer.SyncCreate(prompt); err != nil {
			s.logger.Printf("slash command sync failed: %v", err)
		}
	}

	s.sendToolSuccess(id, fmt.Sprintf("Successfully created prompt: %s", params.Name))

	// Notify clients that the prompt list has changed
	s.sendPromptsListChangedNotification()
}

// callUpdatePromptTool executes the update_prompt tool.
// Converts map arguments to UpdatePromptRequest and delegates to store.Update.
func (s *Server) callUpdatePromptTool(id any, args map[string]interface{}) {
	// Convert map to UpdatePromptRequest
	params, err := convertToUpdatePromptRequest(args)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Invalid arguments: %v", err))
		return
	}

	if params.Name == "" {
		s.sendToolError(id, "Prompt name is required")
		return
	}

	// Get existing prompt
	existing, err := s.store.Get(params.Name)
	if err != nil {
		s.logger.Printf("get prompt error: %v", err)
		s.sendToolError(id, fmt.Sprintf("Prompt not found: %s", params.Name))
		return
	}

	// Apply updates
	applyPromptUpdates(existing, params)

	if err := s.store.Update(existing); err != nil {
		s.logger.Printf("update prompt error: %v", err)
		s.sendToolError(id, fmt.Sprintf("Failed to update prompt: %v", err))
		return
	}

	s.logger.Printf("updated prompt via tool: %s", params.Name)

	// Sync to slash commands if enabled
	if s.syncer != nil {
		if err := s.syncer.SyncUpdate(existing); err != nil {
			s.logger.Printf("slash command sync failed: %v", err)
		}
	}

	s.sendToolSuccess(id, fmt.Sprintf("Successfully updated prompt: %s", params.Name))

	// Notify clients that the prompt list has changed
	s.sendPromptsListChangedNotification()
}

// callDeletePromptTool executes the delete_prompt tool.
// Extracts name from arguments and delegates to store.Delete.
func (s *Server) callDeletePromptTool(id any, args map[string]interface{}) {
	// Extract name from arguments
	name, ok := args["name"].(string)
	if !ok || name == "" {
		s.sendToolError(id, "Prompt name is required")
		return
	}

	if err := s.store.Delete(name); err != nil {
		s.logger.Printf("delete prompt error: %v", err)
		s.sendToolError(id, fmt.Sprintf("Failed to delete prompt: %v", err))
		return
	}

	s.logger.Printf("deleted prompt via tool: %s", name)

	// Delete slash command file if enabled
	if s.syncer != nil {
		if err := s.syncer.Delete(name); err != nil {
			s.logger.Printf("slash command sync delete failed: %v", err)
		}
	}

	s.sendToolSuccess(id, fmt.Sprintf("Successfully deleted prompt: %s", name))

	// Notify clients that the prompt list has changed
	s.sendPromptsListChangedNotification()
}

// buildArgumentArraySchema creates the JSON Schema for prompt arguments array.
func buildArgumentArraySchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"description": "Template variables that can be substituted when invoking the prompt",
		"items": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Argument name (used in {{name}} placeholders)",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Description of the argument's purpose",
				},
				"required": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this argument must be provided",
				},
			},
			"required": []string{"name"},
		},
	}
}

// buildMessageArraySchema creates the JSON Schema for prompt messages array.
func buildMessageArraySchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"description": "Array of message objects (role + content) that form the prompt",
		"items": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"role": map[string]interface{}{
					"type":        "string",
					"description": "Message role: 'user' or 'assistant'",
					"enum":        []string{"user", "assistant"},
				},
				"content": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{
							"type":        "string",
							"description": "Content type (currently only 'text' is supported)",
							"enum":        []string{"text"},
						},
						"text": map[string]interface{}{
							"type":        "string",
							"description": "The message text (can include {{arg}} placeholders)",
						},
					},
					"required": []string{"type", "text"},
				},
			},
			"required": []string{"role", "content"},
		},
	}
}
