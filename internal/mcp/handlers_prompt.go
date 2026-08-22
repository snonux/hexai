// MCP prompt-related handler methods for list, get, create, update, and delete operations.
package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/snonux/hexai/internal/promptstore"
)

// handlePromptsList processes the prompts/list request.
func (s *Server) handlePromptsList(req Request) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		s.sendError(req.ID, ErrCodeInvalidRequest, "Server not initialized")
		return
	}
	s.mu.RUnlock()

	var params ListPromptsRequest
	if req.Params != nil && len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, ErrCodeInvalidParams, "Invalid prompts/list params")
			return
		}
	}

	// List prompts from store
	prompts, nextCursor, err := s.store.List(params.Cursor, 100)
	if err != nil {
		s.logger.Printf("store list error: %v", err)
		s.sendError(req.ID, ErrCodeInternalError, "Failed to list prompts")
		return
	}

	// Convert to PromptInfo (initialize as empty slice so JSON marshals as [] not null)
	infos := make([]PromptInfo, 0, len(prompts))
	for _, p := range prompts {
		args := make([]PromptArgument, len(p.Arguments))
		for i, a := range p.Arguments {
			args[i] = PromptArgument{
				Name:        a.Name,
				Description: a.Description,
				Required:    a.Required,
			}
		}
		infos = append(infos, PromptInfo{
			Name:        p.Name,
			Title:       p.Title,
			Description: p.Description,
			Arguments:   args,
		})
	}

	result := ListPromptsResult{
		Prompts:    infos,
		NextCursor: nextCursor,
	}

	s.sendResponse(req.ID, result)
}

// handlePromptsGet processes the prompts/get request.
func (s *Server) handlePromptsGet(req Request) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		s.sendError(req.ID, ErrCodeInvalidRequest, "Server not initialized")
		return
	}
	s.mu.RUnlock()

	var params GetPromptRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid prompts/get params")
		return
	}

	if params.Name == "" {
		s.sendError(req.ID, ErrCodeInvalidParams, "Missing prompt name")
		return
	}

	// Get prompt from store
	prompt, err := s.store.Get(params.Name)
	if err != nil {
		s.logger.Printf("store get error: %v", err)
		s.sendError(req.ID, ErrCodeInvalidParams, fmt.Sprintf("Prompt not found: %s", params.Name))
		return
	}

	// Render prompt with arguments
	messages, err := s.renderPrompt(prompt, params.Arguments)
	if err != nil {
		s.logger.Printf("render error: %v", err)
		s.sendError(req.ID, ErrCodeInvalidParams, err.Error())
		return
	}

	result := GetPromptResult{
		Description: prompt.Description,
		Messages:    messages,
	}

	s.sendResponse(req.ID, result)
}

// renderPrompt substitutes template arguments in prompt messages.
// Returns error if required arguments are missing.
func (s *Server) renderPrompt(prompt *promptstore.Prompt, args map[string]string) ([]PromptMessage, error) {
	// Validate required arguments
	for _, arg := range prompt.Arguments {
		if arg.Required {
			if _, ok := args[arg.Name]; !ok {
				return nil, fmt.Errorf("missing required argument: %s", arg.Name)
			}
		}
	}

	// Render each message by substituting {{arg}} placeholders with values
	var rendered []PromptMessage
	for _, msg := range prompt.Messages {
		text := msg.Content.Text

		for key, val := range args {
			placeholder := "{{" + key + "}}"
			text = strings.ReplaceAll(text, placeholder, val)
		}

		rendered = append(rendered, PromptMessage{
			Role: msg.Role,
			Content: MessageContent{
				Type: msg.Content.Type,
				Text: text,
			},
		})
	}

	return rendered, nil
}

// handlePromptsCreate processes the prompts/create request.
// Creates a new custom prompt in user.jsonl.
func (s *Server) handlePromptsCreate(req Request) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		s.sendError(req.ID, ErrCodeInvalidRequest, "Server not initialized")
		return
	}
	s.mu.RUnlock()

	var params CreatePromptRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid prompts/create params")
		return
	}

	// Validate required fields
	if err := validateCreateParams(params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, err.Error())
		return
	}

	// Build prompt from params
	prompt := buildPromptFromCreateParams(params)

	if err := s.store.Create(prompt); err != nil {
		s.logger.Printf("create prompt error: %v", err)
		s.sendError(req.ID, ErrCodeInternalError, fmt.Sprintf("Failed to create prompt: %v", err))
		return
	}

	result := PromptOperationResult{
		Success: true,
		Message: fmt.Sprintf("Created prompt: %s", params.Name),
	}

	s.logger.Printf("created prompt: %s", params.Name)

	// Sync to slash commands if enabled
	if s.syncer != nil {
		if err := s.syncer.SyncCreate(prompt); err != nil {
			s.logger.Printf("slash command sync failed: %v", err)
		}
	}

	s.sendResponse(req.ID, result)

	// Notify clients that the prompt list has changed
	s.sendPromptsListChangedNotification()
}

// validateCreateParams validates required fields for prompt creation.
func validateCreateParams(params CreatePromptRequest) error {
	if params.Name == "" {
		return fmt.Errorf("prompt name is required")
	}
	if params.Title == "" {
		return fmt.Errorf("prompt title is required")
	}
	if len(params.Messages) == 0 {
		return fmt.Errorf("at least one message is required")
	}
	return nil
}

// buildPromptFromCreateParams converts CreatePromptRequest to Prompt.
func buildPromptFromCreateParams(params CreatePromptRequest) *promptstore.Prompt {
	prompt := &promptstore.Prompt{
		Name:        params.Name,
		Title:       params.Title,
		Description: params.Description,
		Tags:        params.Tags,
		Created:     time.Now(),
		Updated:     time.Now(),
	}

	// Convert arguments
	for _, arg := range params.Arguments {
		prompt.Arguments = append(prompt.Arguments, promptstore.PromptArgument{
			Name:        arg.Name,
			Description: arg.Description,
			Required:    arg.Required,
		})
	}

	// Convert messages
	for _, msg := range params.Messages {
		prompt.Messages = append(prompt.Messages, promptstore.PromptMessage{
			Role: msg.Role,
			Content: promptstore.MessageContent{
				Type: msg.Content.Type,
				Text: msg.Content.Text,
			},
		})
	}

	return prompt
}

// handlePromptsUpdate processes the prompts/update request.
// Updates an existing custom prompt in user.jsonl.
func (s *Server) handlePromptsUpdate(req Request) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		s.sendError(req.ID, ErrCodeInvalidRequest, "Server not initialized")
		return
	}
	s.mu.RUnlock()

	var params UpdatePromptRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid prompts/update params")
		return
	}

	if params.Name == "" {
		s.sendError(req.ID, ErrCodeInvalidParams, "Prompt name is required")
		return
	}

	// Get existing prompt
	existing, err := s.store.Get(params.Name)
	if err != nil {
		s.logger.Printf("get prompt error: %v", err)
		s.sendError(req.ID, ErrCodeInvalidParams, fmt.Sprintf("Prompt not found: %s", params.Name))
		return
	}

	// Apply updates to existing prompt
	applyPromptUpdates(existing, params)

	if err := s.store.Update(existing); err != nil {
		s.logger.Printf("update prompt error: %v", err)
		s.sendError(req.ID, ErrCodeInternalError, fmt.Sprintf("Failed to update prompt: %v", err))
		return
	}

	result := PromptOperationResult{
		Success: true,
		Message: fmt.Sprintf("Updated prompt: %s", params.Name),
	}

	s.logger.Printf("updated prompt: %s", params.Name)

	// Sync to slash commands if enabled
	if s.syncer != nil {
		if err := s.syncer.SyncUpdate(existing); err != nil {
			s.logger.Printf("slash command sync failed: %v", err)
		}
	}

	s.sendResponse(req.ID, result)

	// Notify clients that the prompt list has changed
	s.sendPromptsListChangedNotification()
}

// applyPromptUpdates applies update parameters to an existing prompt.
// Only non-empty fields are applied; empty fields are left unchanged.
func applyPromptUpdates(existing *promptstore.Prompt, params UpdatePromptRequest) {
	if params.Title != "" {
		existing.Title = params.Title
	}
	if params.Description != "" {
		existing.Description = params.Description
	}
	if len(params.Arguments) > 0 {
		existing.Arguments = nil
		for _, arg := range params.Arguments {
			existing.Arguments = append(existing.Arguments, promptstore.PromptArgument{
				Name:        arg.Name,
				Description: arg.Description,
				Required:    arg.Required,
			})
		}
	}
	if len(params.Messages) > 0 {
		existing.Messages = nil
		for _, msg := range params.Messages {
			existing.Messages = append(existing.Messages, promptstore.PromptMessage{
				Role: msg.Role,
				Content: promptstore.MessageContent{
					Type: msg.Content.Type,
					Text: msg.Content.Text,
				},
			})
		}
	}
	if len(params.Tags) > 0 {
		existing.Tags = params.Tags
	}

	existing.Updated = time.Now()
}

// handlePromptsDelete processes the prompts/delete request.
// Deletes a custom prompt from user.jsonl.
func (s *Server) handlePromptsDelete(req Request) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		s.sendError(req.ID, ErrCodeInvalidRequest, "Server not initialized")
		return
	}
	s.mu.RUnlock()

	var params DeletePromptRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid prompts/delete params")
		return
	}

	if params.Name == "" {
		s.sendError(req.ID, ErrCodeInvalidParams, "Prompt name is required")
		return
	}

	if err := s.store.Delete(params.Name); err != nil {
		s.logger.Printf("delete prompt error: %v", err)
		s.sendError(req.ID, ErrCodeInternalError, fmt.Sprintf("Failed to delete prompt: %v", err))
		return
	}

	result := PromptOperationResult{
		Success: true,
		Message: fmt.Sprintf("Deleted prompt: %s", params.Name),
	}

	s.logger.Printf("deleted prompt: %s", params.Name)

	// Delete slash command file if enabled
	if s.syncer != nil {
		if err := s.syncer.Delete(params.Name); err != nil {
			s.logger.Printf("slash command sync delete failed: %v", err)
		}
	}

	s.sendResponse(req.ID, result)

	// Notify clients that the prompt list has changed
	s.sendPromptsListChangedNotification()
}

// convertToCreatePromptRequest converts map arguments to CreatePromptRequest.
// Uses JSON marshal/unmarshal round-trip for type conversion.
func convertToCreatePromptRequest(args map[string]interface{}) (CreatePromptRequest, error) {
	data, err := json.Marshal(args)
	if err != nil {
		return CreatePromptRequest{}, fmt.Errorf("marshal args: %w", err)
	}

	var req CreatePromptRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return CreatePromptRequest{}, fmt.Errorf("unmarshal to CreatePromptRequest: %w", err)
	}

	return req, nil
}

// convertToUpdatePromptRequest converts map arguments to UpdatePromptRequest.
// Uses JSON marshal/unmarshal round-trip for type conversion.
func convertToUpdatePromptRequest(args map[string]interface{}) (UpdatePromptRequest, error) {
	data, err := json.Marshal(args)
	if err != nil {
		return UpdatePromptRequest{}, fmt.Errorf("marshal args: %w", err)
	}

	var req UpdatePromptRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return UpdatePromptRequest{}, fmt.Errorf("unmarshal to UpdatePromptRequest: %w", err)
	}

	return req, nil
}
