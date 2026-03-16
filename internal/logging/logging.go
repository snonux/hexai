// Package logging provides ANSI-styled logging utilities with a bound standard logger and configurable preview truncation.
// All package-level state is accessed via atomic types to avoid data races under concurrent use.
package logging

import (
	"fmt"
	"log"
	"sync/atomic"
)

// ANSI color utilities shared across Hexai.
const (
	AnsiBgBlack = "\x1b[40m"
	AnsiGrey    = "\x1b[90m"
	AnsiCyan    = "\x1b[36m"
	AnsiGreen   = "\x1b[32m"
	AnsiYellow  = "\x1b[33m"
	AnsiRed     = "\x1b[31m"
	AnsiReset   = "\x1b[0m"
)

// AnsiBase is the default style: black background + grey foreground.
const AnsiBase = AnsiBgBlack + AnsiGrey

// std is the singleton logger used across the codebase, stored atomically for concurrent safety.
var std atomic.Pointer[log.Logger]

// Bind atomically sets the underlying standard logger to use for Logf.
func Bind(l *log.Logger) { std.Store(l) }

// Logf prints a formatted message with a module prefix and base ANSI style.
// It atomically loads the bound logger; if none is set the call is a no-op.
func Logf(prefix, format string, args ...any) {
	l := std.Load()
	if l == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	l.Print(AnsiBase + prefix + msg + AnsiReset)
}

// logPreviewLimit is the max characters logged for request/response previews.
// Stored as atomic.Int32 for concurrent safety; 0 means unlimited.
var logPreviewLimit atomic.Int32

// SetLogPreviewLimit atomically sets the maximum number of characters to log
// for request/response previews. Set to 0 for unlimited.
func SetLogPreviewLimit(n int) { logPreviewLimit.Store(int32(n)) }

// PreviewForLog returns the string truncated to the configured preview limit
// (loaded atomically).
func PreviewForLog(s string) string {
	limit := int(logPreviewLimit.Load())
	if limit > 0 {
		if len(s) <= limit {
			return s
		}
		return s[:limit] + "…"
	}
	return s
}
