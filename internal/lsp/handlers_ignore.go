// Helpers for gitignore-aware file filtering in LSP handlers.

package lsp

import (
	"net/url"
	"strings"
)

// isFileIgnored checks whether the file at the given LSP URI should be ignored.
// Returns false when no ignore checker is configured.
func (s *Server) isFileIgnored(uri string) (bool, string) {
	if s.ignoreChecker == nil {
		return false, ""
	}
	absPath := uriToPath(uri)
	if absPath == "" {
		return false, ""
	}
	return s.ignoreChecker.IsIgnored(absPath)
}

// ignoreLSPNotifyEnabled returns whether to show "file ignored" completion items
// when a file is ignored. Reads from the IgnoreLSPNotify config field.
func (s *Server) ignoreLSPNotifyEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.IgnoreLSPNotify == nil || *s.cfg.IgnoreLSPNotify
}

// uriToPath converts a file:// URI to an absolute file path.
// Returns empty string for non-file URIs.
func uriToPath(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return ""
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	return parsed.Path
}
