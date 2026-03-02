package appconfig

import (
	"slices"
	"strings"
)

func (a *App) mergeWith(other *App) {
	a.mergeBasics(other)
	a.mergeProviderFields(other)
	a.mergeSurfaceModels(other)
	a.mergePrompts(other)
	a.mergeTmuxEdit(other)
}

// mergeBasics merges general (non-provider) fields.
func (a *App) mergeBasics(other *App) {
	mergeBasicCore(a, other)
	mergeBasicCompletion(a, other)
	mergeBasicInteraction(a, other)
	mergeBasicRuntime(a, other)
}

func mergeBasicCore(dst, src *App) {
	if src.MaxTokens > 0 {
		dst.MaxTokens = src.MaxTokens
	}
	if s := strings.TrimSpace(src.ContextMode); s != "" {
		dst.ContextMode = s
	}
	if src.ContextWindowLines > 0 {
		dst.ContextWindowLines = src.ContextWindowLines
	}
	if src.MaxContextTokens > 0 {
		dst.MaxContextTokens = src.MaxContextTokens
	}
	if src.LogPreviewLimit >= 0 {
		dst.LogPreviewLimit = src.LogPreviewLimit
	}
	if src.RequestTimeout > 0 {
		dst.RequestTimeout = src.RequestTimeout
	}
	if src.CodingTemperature != nil { // allow explicit 0.0
		dst.CodingTemperature = src.CodingTemperature
	}
}

func mergeBasicCompletion(dst, src *App) {
	if src.ManualInvokeMinPrefix >= 0 {
		dst.ManualInvokeMinPrefix = src.ManualInvokeMinPrefix
	}
	if src.CompletionDebounceMs > 0 {
		dst.CompletionDebounceMs = src.CompletionDebounceMs
	}
	if src.CompletionThrottleMs > 0 {
		dst.CompletionThrottleMs = src.CompletionThrottleMs
	}
	if src.CompletionWaitAll != nil {
		dst.CompletionWaitAll = src.CompletionWaitAll
	}
}

func mergeBasicInteraction(dst, src *App) {
	if len(src.TriggerCharacters) > 0 {
		dst.TriggerCharacters = slices.Clone(src.TriggerCharacters)
	}
	if s := strings.TrimSpace(src.InlineOpen); s != "" {
		dst.InlineOpen = s
	}
	if s := strings.TrimSpace(src.InlineClose); s != "" {
		dst.InlineClose = s
	}
	if s := strings.TrimSpace(src.ChatSuffix); s != "" {
		dst.ChatSuffix = s
	}
	if len(src.ChatPrefixes) > 0 {
		dst.ChatPrefixes = slices.Clone(src.ChatPrefixes)
	}
	if s := strings.TrimSpace(src.Provider); s != "" {
		dst.Provider = s
	}
}

func mergeBasicRuntime(dst, src *App) {
	if src.IgnoreGitignore != nil {
		dst.IgnoreGitignore = src.IgnoreGitignore
	}
	if len(src.IgnoreExtraPatterns) > 0 {
		dst.IgnoreExtraPatterns = slices.Clone(src.IgnoreExtraPatterns)
	}
	if src.IgnoreLSPNotify != nil {
		dst.IgnoreLSPNotify = src.IgnoreLSPNotify
	}
	if s := strings.TrimSpace(src.MCPPromptsDir); s != "" {
		dst.MCPPromptsDir = s
	}
	if src.MCPSlashCommandSync {
		dst.MCPSlashCommandSync = src.MCPSlashCommandSync
	}
	if s := strings.TrimSpace(src.MCPSlashCommandDir); s != "" {
		dst.MCPSlashCommandDir = s
	}
}

// mergeProviderFields merges per-provider configuration.
func (a *App) mergeProviderFields(other *App) {
	if s := strings.TrimSpace(other.OpenAIBaseURL); s != "" {
		a.OpenAIBaseURL = s
	}
	if s := strings.TrimSpace(other.OpenAIModel); s != "" {
		a.OpenAIModel = s
	}
	if other.OpenAITemperature != nil { // allow explicit 0.0
		a.OpenAITemperature = other.OpenAITemperature
	}
	if s := strings.TrimSpace(other.OpenRouterBaseURL); s != "" {
		a.OpenRouterBaseURL = s
	}
	if s := strings.TrimSpace(other.OpenRouterModel); s != "" {
		a.OpenRouterModel = s
	}
	if other.OpenRouterTemperature != nil { // allow explicit 0.0
		a.OpenRouterTemperature = other.OpenRouterTemperature
	}
	if s := strings.TrimSpace(other.OllamaBaseURL); s != "" {
		a.OllamaBaseURL = s
	}
	if s := strings.TrimSpace(other.OllamaModel); s != "" {
		a.OllamaModel = s
	}
	if other.OllamaTemperature != nil { // allow explicit 0.0
		a.OllamaTemperature = other.OllamaTemperature
	}
	if s := strings.TrimSpace(other.AnthropicBaseURL); s != "" {
		a.AnthropicBaseURL = s
	}
	if s := strings.TrimSpace(other.AnthropicModel); s != "" {
		a.AnthropicModel = s
	}
	if other.AnthropicTemperature != nil { // allow explicit 0.0
		a.AnthropicTemperature = other.AnthropicTemperature
	}
}

// mergeSurfaceModels copies per-surface model and temperature overrides.
func (a *App) mergeSurfaceModels(other *App) {
	if len(other.CompletionConfigs) > 0 {
		a.CompletionConfigs = cloneSurfaceConfigs(other.CompletionConfigs)
	}
	if len(other.CodeActionConfigs) > 0 {
		a.CodeActionConfigs = cloneSurfaceConfigs(other.CodeActionConfigs)
	}
	if len(other.ChatConfigs) > 0 {
		a.ChatConfigs = cloneSurfaceConfigs(other.ChatConfigs)
	}
	if len(other.CLIConfigs) > 0 {
		a.CLIConfigs = cloneSurfaceConfigs(other.CLIConfigs)
	}
}

func cloneSurfaceConfigs(src []SurfaceConfig) []SurfaceConfig {
	if len(src) == 0 {
		return nil
	}
	out := make([]SurfaceConfig, len(src))
	copy(out, src)
	return out
}

// mergePrompts copies non-empty prompt templates from other.
func (a *App) mergePrompts(other *App) {
	mergeCompletionPrompts(a, other)
	mergeProviderNativePrompt(a, other)
	mergeChatPrompt(a, other)
	mergeCodeActionPrompts(a, other)
	mergeCLIPrompts(a, other)
	mergeCustomActionPrompts(a, other)
}

func mergeCompletionPrompts(dst, src *App) {
	mergeStringField(&dst.PromptCompletionSystemGeneral, src.PromptCompletionSystemGeneral)
	mergeStringField(&dst.PromptCompletionSystemParams, src.PromptCompletionSystemParams)
	mergeStringField(&dst.PromptCompletionSystemInline, src.PromptCompletionSystemInline)
	mergeStringField(&dst.PromptCompletionUserGeneral, src.PromptCompletionUserGeneral)
	mergeStringField(&dst.PromptCompletionUserParams, src.PromptCompletionUserParams)
	mergeStringField(&dst.PromptCompletionExtraHeader, src.PromptCompletionExtraHeader)
}

func mergeProviderNativePrompt(dst, src *App) {
	mergeStringField(&dst.PromptNativeCompletion, src.PromptNativeCompletion)
}

func mergeChatPrompt(dst, src *App) {
	mergeStringField(&dst.PromptChatSystem, src.PromptChatSystem)
}

func mergeCodeActionPrompts(dst, src *App) {
	mergeStringField(&dst.PromptCodeActionRewriteSystem, src.PromptCodeActionRewriteSystem)
	mergeStringField(&dst.PromptCodeActionDiagnosticsSystem, src.PromptCodeActionDiagnosticsSystem)
	mergeStringField(&dst.PromptCodeActionDocumentSystem, src.PromptCodeActionDocumentSystem)
	mergeStringField(&dst.PromptCodeActionRewriteUser, src.PromptCodeActionRewriteUser)
	mergeStringField(&dst.PromptCodeActionDiagnosticsUser, src.PromptCodeActionDiagnosticsUser)
	mergeStringField(&dst.PromptCodeActionDocumentUser, src.PromptCodeActionDocumentUser)
	mergeStringField(&dst.PromptCodeActionGoTestSystem, src.PromptCodeActionGoTestSystem)
	mergeStringField(&dst.PromptCodeActionGoTestUser, src.PromptCodeActionGoTestUser)
	mergeStringField(&dst.PromptCodeActionSimplifySystem, src.PromptCodeActionSimplifySystem)
	mergeStringField(&dst.PromptCodeActionSimplifyUser, src.PromptCodeActionSimplifyUser)
}

func mergeCLIPrompts(dst, src *App) {
	mergeStringField(&dst.PromptCLIDefaultSystem, src.PromptCLIDefaultSystem)
	mergeStringField(&dst.PromptCLIExplainSystem, src.PromptCLIExplainSystem)
}

func mergeCustomActionPrompts(dst, src *App) {
	if len(src.CustomActions) > 0 {
		dst.CustomActions = append([]CustomAction{}, src.CustomActions...)
	}
	mergeStringField(&dst.TmuxCustomMenuHotkey, src.TmuxCustomMenuHotkey)
}

func mergeStringField(dst *string, src string) {
	if strings.TrimSpace(src) != "" {
		*dst = src
	}
}

// mergeTmuxEdit copies non-empty tmux edit settings from other.
func (a *App) mergeTmuxEdit(other *App) {
	if s := strings.TrimSpace(other.TmuxEditPopupWidth); s != "" {
		a.TmuxEditPopupWidth = s
	}
	if s := strings.TrimSpace(other.TmuxEditPopupHeight); s != "" {
		a.TmuxEditPopupHeight = s
	}
	if s := strings.TrimSpace(other.TmuxEditDefaultAgent); s != "" {
		a.TmuxEditDefaultAgent = s
	}
	if len(other.TmuxEditAgents) > 0 {
		a.TmuxEditAgents = append([]TmuxEditAgentCfg{}, other.TmuxEditAgents...)
	}
}
