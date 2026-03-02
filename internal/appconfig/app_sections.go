package appconfig

import "slices"

// CoreConfig contains core runtime and interaction settings.
type CoreConfig struct {
	MaxTokens             int
	ContextMode           string
	ContextWindowLines    int
	MaxContextTokens      int
	LogPreviewLimit       int
	RequestTimeout        int
	CodingTemperature     *float64
	ManualInvokeMinPrefix int
	CompletionDebounceMs  int
	CompletionThrottleMs  int
	CompletionWaitAll     *bool
	TriggerCharacters     []string
	Provider              string
	InlineOpen            string
	InlineClose           string
	ChatSuffix            string
	ChatPrefixes          []string
}

// ProviderConfig contains provider endpoints/models and per-surface model overrides.
type ProviderConfig struct {
	OpenAIBaseURL         string
	OpenAIModel           string
	OpenAITemperature     *float64
	OpenRouterBaseURL     string
	OpenRouterModel       string
	OpenRouterTemperature *float64
	OllamaBaseURL         string
	OllamaModel           string
	OllamaTemperature     *float64
	AnthropicBaseURL      string
	AnthropicModel        string
	AnthropicTemperature  *float64
	CompletionConfigs     []SurfaceConfig
	CodeActionConfigs     []SurfaceConfig
	ChatConfigs           []SurfaceConfig
	CLIConfigs            []SurfaceConfig
}

// PromptConfig contains all prompt templates and custom action prompts.
type PromptConfig struct {
	PromptCompletionSystemGeneral     string
	PromptCompletionSystemParams      string
	PromptCompletionSystemInline      string
	PromptCompletionUserGeneral       string
	PromptCompletionUserParams        string
	PromptCompletionExtraHeader       string
	PromptNativeCompletion            string
	PromptChatSystem                  string
	PromptCodeActionRewriteSystem     string
	PromptCodeActionDiagnosticsSystem string
	PromptCodeActionDocumentSystem    string
	PromptCodeActionRewriteUser       string
	PromptCodeActionDiagnosticsUser   string
	PromptCodeActionDocumentUser      string
	PromptCodeActionGoTestSystem      string
	PromptCodeActionGoTestUser        string
	PromptCodeActionSimplifySystem    string
	PromptCodeActionSimplifyUser      string
	PromptCLIDefaultSystem            string
	PromptCLIExplainSystem            string
	CustomActions                     []CustomAction
	TmuxCustomMenuHotkey              string
}

// FeatureConfig contains non-LLM feature toggles/integration settings.
type FeatureConfig struct {
	StatsWindowMinutes   int
	IgnoreGitignore      *bool
	IgnoreExtraPatterns  []string
	IgnoreLSPNotify      *bool
	TmuxEditPopupWidth   string
	TmuxEditPopupHeight  string
	TmuxEditDefaultAgent string
	TmuxEditAgents       []TmuxEditAgentCfg
	MCPPromptsDir        string
	MCPSlashCommandSync  bool
	MCPSlashCommandDir   string
}

// AppSections is the focused split of App into subsystem-specific config groups.
type AppSections struct {
	Core      CoreConfig
	Providers ProviderConfig
	Prompts   PromptConfig
	Features  FeatureConfig
}

// Sections returns the app configuration split into focused sub-configs.
func (a App) Sections() AppSections {
	return AppSections{
		Core:      a.CoreSection(),
		Providers: a.ProviderSection(),
		Prompts:   a.PromptSection(),
		Features:  a.FeatureSection(),
	}
}

// ApplySections applies focused sub-config groups back onto App.
func (a *App) ApplySections(sections AppSections) {
	a.ApplyCoreSection(sections.Core)
	a.ApplyProviderSection(sections.Providers)
	a.ApplyPromptSection(sections.Prompts)
	a.ApplyFeatureSection(sections.Features)
}

// CoreSection returns the core runtime and interaction settings.
func (a App) CoreSection() CoreConfig {
	return CoreConfig{
		MaxTokens:             a.MaxTokens,
		ContextMode:           a.ContextMode,
		ContextWindowLines:    a.ContextWindowLines,
		MaxContextTokens:      a.MaxContextTokens,
		LogPreviewLimit:       a.LogPreviewLimit,
		RequestTimeout:        a.RequestTimeout,
		CodingTemperature:     a.CodingTemperature,
		ManualInvokeMinPrefix: a.ManualInvokeMinPrefix,
		CompletionDebounceMs:  a.CompletionDebounceMs,
		CompletionThrottleMs:  a.CompletionThrottleMs,
		CompletionWaitAll:     a.CompletionWaitAll,
		TriggerCharacters:     slices.Clone(a.TriggerCharacters),
		Provider:              a.Provider,
		InlineOpen:            a.InlineOpen,
		InlineClose:           a.InlineClose,
		ChatSuffix:            a.ChatSuffix,
		ChatPrefixes:          slices.Clone(a.ChatPrefixes),
	}
}

// ApplyCoreSection applies core runtime and interaction settings.
func (a *App) ApplyCoreSection(core CoreConfig) {
	a.MaxTokens = core.MaxTokens
	a.ContextMode = core.ContextMode
	a.ContextWindowLines = core.ContextWindowLines
	a.MaxContextTokens = core.MaxContextTokens
	a.LogPreviewLimit = core.LogPreviewLimit
	a.RequestTimeout = core.RequestTimeout
	a.CodingTemperature = core.CodingTemperature
	a.ManualInvokeMinPrefix = core.ManualInvokeMinPrefix
	a.CompletionDebounceMs = core.CompletionDebounceMs
	a.CompletionThrottleMs = core.CompletionThrottleMs
	a.CompletionWaitAll = core.CompletionWaitAll
	a.TriggerCharacters = slices.Clone(core.TriggerCharacters)
	a.Provider = core.Provider
	a.InlineOpen = core.InlineOpen
	a.InlineClose = core.InlineClose
	a.ChatSuffix = core.ChatSuffix
	a.ChatPrefixes = slices.Clone(core.ChatPrefixes)
}

// ProviderSection returns provider endpoint/model settings and surface overrides.
func (a App) ProviderSection() ProviderConfig {
	return ProviderConfig{
		OpenAIBaseURL:         a.OpenAIBaseURL,
		OpenAIModel:           a.OpenAIModel,
		OpenAITemperature:     a.OpenAITemperature,
		OpenRouterBaseURL:     a.OpenRouterBaseURL,
		OpenRouterModel:       a.OpenRouterModel,
		OpenRouterTemperature: a.OpenRouterTemperature,
		OllamaBaseURL:         a.OllamaBaseURL,
		OllamaModel:           a.OllamaModel,
		OllamaTemperature:     a.OllamaTemperature,
		AnthropicBaseURL:      a.AnthropicBaseURL,
		AnthropicModel:        a.AnthropicModel,
		AnthropicTemperature:  a.AnthropicTemperature,
		CompletionConfigs:     cloneSurfaceConfigs(a.CompletionConfigs),
		CodeActionConfigs:     cloneSurfaceConfigs(a.CodeActionConfigs),
		ChatConfigs:           cloneSurfaceConfigs(a.ChatConfigs),
		CLIConfigs:            cloneSurfaceConfigs(a.CLIConfigs),
	}
}

// ApplyProviderSection applies provider endpoint/model settings and surface overrides.
func (a *App) ApplyProviderSection(providers ProviderConfig) {
	a.OpenAIBaseURL = providers.OpenAIBaseURL
	a.OpenAIModel = providers.OpenAIModel
	a.OpenAITemperature = providers.OpenAITemperature
	a.OpenRouterBaseURL = providers.OpenRouterBaseURL
	a.OpenRouterModel = providers.OpenRouterModel
	a.OpenRouterTemperature = providers.OpenRouterTemperature
	a.OllamaBaseURL = providers.OllamaBaseURL
	a.OllamaModel = providers.OllamaModel
	a.OllamaTemperature = providers.OllamaTemperature
	a.AnthropicBaseURL = providers.AnthropicBaseURL
	a.AnthropicModel = providers.AnthropicModel
	a.AnthropicTemperature = providers.AnthropicTemperature
	a.CompletionConfigs = cloneSurfaceConfigs(providers.CompletionConfigs)
	a.CodeActionConfigs = cloneSurfaceConfigs(providers.CodeActionConfigs)
	a.ChatConfigs = cloneSurfaceConfigs(providers.ChatConfigs)
	a.CLIConfigs = cloneSurfaceConfigs(providers.CLIConfigs)
}

// PromptSection returns prompt templates and custom action prompt settings.
func (a App) PromptSection() PromptConfig {
	return PromptConfig{
		PromptCompletionSystemGeneral:     a.PromptCompletionSystemGeneral,
		PromptCompletionSystemParams:      a.PromptCompletionSystemParams,
		PromptCompletionSystemInline:      a.PromptCompletionSystemInline,
		PromptCompletionUserGeneral:       a.PromptCompletionUserGeneral,
		PromptCompletionUserParams:        a.PromptCompletionUserParams,
		PromptCompletionExtraHeader:       a.PromptCompletionExtraHeader,
		PromptNativeCompletion:            a.PromptNativeCompletion,
		PromptChatSystem:                  a.PromptChatSystem,
		PromptCodeActionRewriteSystem:     a.PromptCodeActionRewriteSystem,
		PromptCodeActionDiagnosticsSystem: a.PromptCodeActionDiagnosticsSystem,
		PromptCodeActionDocumentSystem:    a.PromptCodeActionDocumentSystem,
		PromptCodeActionRewriteUser:       a.PromptCodeActionRewriteUser,
		PromptCodeActionDiagnosticsUser:   a.PromptCodeActionDiagnosticsUser,
		PromptCodeActionDocumentUser:      a.PromptCodeActionDocumentUser,
		PromptCodeActionGoTestSystem:      a.PromptCodeActionGoTestSystem,
		PromptCodeActionGoTestUser:        a.PromptCodeActionGoTestUser,
		PromptCodeActionSimplifySystem:    a.PromptCodeActionSimplifySystem,
		PromptCodeActionSimplifyUser:      a.PromptCodeActionSimplifyUser,
		PromptCLIDefaultSystem:            a.PromptCLIDefaultSystem,
		PromptCLIExplainSystem:            a.PromptCLIExplainSystem,
		CustomActions:                     append([]CustomAction{}, a.CustomActions...),
		TmuxCustomMenuHotkey:              a.TmuxCustomMenuHotkey,
	}
}

// ApplyPromptSection applies prompt templates and custom action prompt settings.
func (a *App) ApplyPromptSection(prompts PromptConfig) {
	a.PromptCompletionSystemGeneral = prompts.PromptCompletionSystemGeneral
	a.PromptCompletionSystemParams = prompts.PromptCompletionSystemParams
	a.PromptCompletionSystemInline = prompts.PromptCompletionSystemInline
	a.PromptCompletionUserGeneral = prompts.PromptCompletionUserGeneral
	a.PromptCompletionUserParams = prompts.PromptCompletionUserParams
	a.PromptCompletionExtraHeader = prompts.PromptCompletionExtraHeader
	a.PromptNativeCompletion = prompts.PromptNativeCompletion
	a.PromptChatSystem = prompts.PromptChatSystem
	a.PromptCodeActionRewriteSystem = prompts.PromptCodeActionRewriteSystem
	a.PromptCodeActionDiagnosticsSystem = prompts.PromptCodeActionDiagnosticsSystem
	a.PromptCodeActionDocumentSystem = prompts.PromptCodeActionDocumentSystem
	a.PromptCodeActionRewriteUser = prompts.PromptCodeActionRewriteUser
	a.PromptCodeActionDiagnosticsUser = prompts.PromptCodeActionDiagnosticsUser
	a.PromptCodeActionDocumentUser = prompts.PromptCodeActionDocumentUser
	a.PromptCodeActionGoTestSystem = prompts.PromptCodeActionGoTestSystem
	a.PromptCodeActionGoTestUser = prompts.PromptCodeActionGoTestUser
	a.PromptCodeActionSimplifySystem = prompts.PromptCodeActionSimplifySystem
	a.PromptCodeActionSimplifyUser = prompts.PromptCodeActionSimplifyUser
	a.PromptCLIDefaultSystem = prompts.PromptCLIDefaultSystem
	a.PromptCLIExplainSystem = prompts.PromptCLIExplainSystem
	a.CustomActions = append([]CustomAction{}, prompts.CustomActions...)
	a.TmuxCustomMenuHotkey = prompts.TmuxCustomMenuHotkey
}

// FeatureSection returns non-LLM feature toggles and integrations.
func (a App) FeatureSection() FeatureConfig {
	return FeatureConfig{
		StatsWindowMinutes:   a.StatsWindowMinutes,
		IgnoreGitignore:      a.IgnoreGitignore,
		IgnoreExtraPatterns:  slices.Clone(a.IgnoreExtraPatterns),
		IgnoreLSPNotify:      a.IgnoreLSPNotify,
		TmuxEditPopupWidth:   a.TmuxEditPopupWidth,
		TmuxEditPopupHeight:  a.TmuxEditPopupHeight,
		TmuxEditDefaultAgent: a.TmuxEditDefaultAgent,
		TmuxEditAgents:       append([]TmuxEditAgentCfg{}, a.TmuxEditAgents...),
		MCPPromptsDir:        a.MCPPromptsDir,
		MCPSlashCommandSync:  a.MCPSlashCommandSync,
		MCPSlashCommandDir:   a.MCPSlashCommandDir,
	}
}

// ApplyFeatureSection applies non-LLM feature toggles and integrations.
func (a *App) ApplyFeatureSection(features FeatureConfig) {
	a.StatsWindowMinutes = features.StatsWindowMinutes
	a.IgnoreGitignore = features.IgnoreGitignore
	a.IgnoreExtraPatterns = slices.Clone(features.IgnoreExtraPatterns)
	a.IgnoreLSPNotify = features.IgnoreLSPNotify
	a.TmuxEditPopupWidth = features.TmuxEditPopupWidth
	a.TmuxEditPopupHeight = features.TmuxEditPopupHeight
	a.TmuxEditDefaultAgent = features.TmuxEditDefaultAgent
	a.TmuxEditAgents = append([]TmuxEditAgentCfg{}, features.TmuxEditAgents...)
	a.MCPPromptsDir = features.MCPPromptsDir
	a.MCPSlashCommandSync = features.MCPSlashCommandSync
	a.MCPSlashCommandDir = features.MCPSlashCommandDir
}
