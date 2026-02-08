// Summary: Thread-safe gitignore-aware file checker that combines .gitignore
// patterns with user-configured extra patterns. Used by the LSP server to
// skip completions and code actions for ignored files.
package ignore

import (
	"path/filepath"
	"strings"
	"sync"

	gitignore "github.com/sabhiram/go-gitignore"
)

// Checker evaluates whether an absolute file path should be ignored based on
// .gitignore patterns and/or user-configured extra patterns. It is safe for
// concurrent use.
type Checker struct {
	mu        sync.RWMutex
	gitRoot   string
	giMatcher *gitignore.GitIgnore // compiled .gitignore (nil when disabled or missing)
	exMatcher *gitignore.GitIgnore // compiled extra patterns (nil when empty)
}

// New creates a Checker. If useGitignore is true and gitRoot is non-empty, it
// loads .gitignore from gitRoot. extraPatterns are always compiled (gitignore
// syntax).
func New(gitRoot string, useGitignore bool, extraPatterns []string) *Checker {
	c := &Checker{gitRoot: gitRoot}
	c.compile(useGitignore, extraPatterns)
	return c
}

// IsIgnored returns whether absPath should be ignored and a human-readable
// reason string. When the checker is nil, nothing is ignored.
func (c *Checker) IsIgnored(absPath string) (ignored bool, reason string) {
	if c == nil {
		return false, ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	rel, inside := c.relPath(absPath)

	// Only check gitignore when the path is inside the git root
	if inside && c.giMatcher != nil && c.giMatcher.MatchesPath(rel) {
		return true, "matched .gitignore pattern"
	}
	if c.exMatcher != nil && c.exMatcher.MatchesPath(rel) {
		return true, "matched extra ignore pattern"
	}
	return false, ""
}

// Update recompiles matchers for hot-reload. Thread-safe.
func (c *Checker) Update(useGitignore bool, extraPatterns []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.compile(useGitignore, extraPatterns)
}

// compile builds the gitignore and extra-pattern matchers. Must be called
// under c.mu write lock (or during construction).
func (c *Checker) compile(useGitignore bool, extraPatterns []string) {
	c.giMatcher = nil
	c.exMatcher = nil

	if useGitignore && c.gitRoot != "" {
		giPath := filepath.Join(c.gitRoot, ".gitignore")
		if gi, err := gitignore.CompileIgnoreFile(giPath); err == nil {
			c.giMatcher = gi
		}
	}
	if len(extraPatterns) > 0 {
		c.exMatcher = gitignore.CompileIgnoreLines(extraPatterns...)
	}
}

// relPath converts absPath to a path relative to gitRoot. Returns the
// relative path and true if the path is inside the git root; otherwise
// returns the original path and false.
func (c *Checker) relPath(absPath string) (string, bool) {
	if c.gitRoot == "" {
		return absPath, false
	}
	rel, err := filepath.Rel(c.gitRoot, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return absPath, false
	}
	return rel, true
}
