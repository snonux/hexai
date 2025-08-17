// Summary: ANSI-styled logging utilities with a bound standard logger and configurable preview truncation.
package logging

import (
	"fmt"
	"log"
)

// ANSI color utilities shared across Hexai.
const (
	AnsiBgBlack = "\x1b[40m"
	AnsiGrey    = "\x1b[90m"
	AnsiCyan    = "\x1b[36m"
	AnsiGreen   = "\x1b[32m"
	AnsiRed     = "\x1b[31m"
	AnsiReset   = "\x1b[0m"
)

// AnsiBase is the default style: black background + grey foreground.
const AnsiBase = AnsiBgBlack + AnsiGrey

// singleton logger used across the codebase
var std *log.Logger

// Bind sets the underlying standard logger to use for Logf.
func Bind(l *log.Logger) { std = l }

// Logf prints a formatted message with a module prefix and base ANSI style.
func Logf(prefix, format string, args ...any) {
	if std == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	std.Print(AnsiBase + prefix + msg + AnsiReset)
}

// Logging configuration for previews (shared)
var logPreviewLimit int // 0 means unlimited

// SetLogPreviewLimit sets the maximum number of characters to log for
// request/response previews. Set to 0 for unlimited.
func SetLogPreviewLimit(n int) { logPreviewLimit = n }

// PreviewForLog returns the string truncated to the configured preview limit.
func PreviewForLog(s string) string {
	if logPreviewLimit > 0 {
		if len(s) <= logPreviewLimit {
			return s
		}
		return s[:logPreviewLimit] + "…"
	}
	return s
}
