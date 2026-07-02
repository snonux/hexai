package appconfig

import "slices"

// This file provides fine-grained, read-only accessors for the individual
// feature subsystems embedded in FeatureConfig. They let a consumer depend on
// just the subsystem config it needs (e.g. the MCP server only needs MCPConfig)
// instead of receiving the whole App God-struct. Each accessor returns a deep
// copy so callers cannot mutate App's internal slices.

// IgnoreSection returns a copy of the gitignore-aware filtering settings.
func (a *App) IgnoreSection() IgnoreConfig {
	c := a.IgnoreConfig
	c.IgnoreExtraPatterns = slices.Clone(a.IgnoreExtraPatterns)
	return c
}

// StatsSection returns the usage statistics settings.
func (a *App) StatsSection() StatsConfig {
	return a.StatsConfig
}

// TmuxActionSection returns a copy of the tmux action menu settings.
func (a *App) TmuxActionSection() TmuxActionConfig {
	c := a.TmuxActionConfig
	c.TmuxActionMenu = append([]TmuxActionMenuEntry{}, a.TmuxActionMenu...)
	return c
}

// MCPSection returns the Model Context Protocol server settings.
func (a *App) MCPSection() MCPConfig {
	return a.MCPConfig
}
