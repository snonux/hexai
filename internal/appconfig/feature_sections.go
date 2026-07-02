package appconfig

// This file defines the cohesive per-subsystem config structs that make up
// FeatureConfig. The old FeatureConfig was a grab-bag that mixed four unrelated
// non-LLM subsystems (ignore filtering, stats, tmux action menu, MCP server).
// Splitting them into named structs documents the seams
// between subsystems and lets consumers depend on a single subsystem's config
// (via the *Section accessors on App) instead of the whole App God-struct.
//
// Each struct is embedded into FeatureConfig, so Go field promotion keeps the
// historical flat access (e.g. cfg.MCPPromptsDir) working for read sites across
// the codebase. Only composite literals that set these leaf fields directly on
// FeatureConfig had to move to the nested form.

// IgnoreConfig controls gitignore-aware file filtering for the LSP server.
// Files matching these patterns are skipped for completions and code actions.
type IgnoreConfig struct {
	// IgnoreGitignore enables respecting .gitignore entries (default true).
	IgnoreGitignore *bool `json:"-"`
	// IgnoreExtraPatterns are additional glob patterns to always ignore.
	IgnoreExtraPatterns []string `json:"-"`
	// IgnoreLSPNotify controls whether the LSP notifies when ignoring a file.
	IgnoreLSPNotify *bool `json:"-"`
}

// StatsConfig holds settings for the usage statistics subsystem.
type StatsConfig struct {
	// StatsWindowMinutes is the rolling window (in minutes) used for stats.
	StatsWindowMinutes int `json:"-"`
}

// TmuxActionConfig configures the main menu for hexai-tmux-action.
type TmuxActionConfig struct {
	TmuxActionMenu []TmuxActionMenuEntry `json:"-"`
}

// MCPConfig holds Model Context Protocol server settings.
type MCPConfig struct {
	MCPPromptsDir       string `json:"-"` // Directory for prompt storage
	MCPSlashCommandSync bool   `json:"-"` // Enable slash command sync
	MCPSlashCommandDir  string `json:"-"` // Directory for slash command files
}
