package llm

import (
    "fmt"
    "log"
)

// ANSI color utilities shared across LLM providers.
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

// LogPrintf wraps a formatted message with a base style and prints with a prefix.
func LogPrintf(logger *log.Logger, prefix, format string, args ...any) {
    if logger == nil {
        return
    }
    msg := fmt.Sprintf(format, args...)
    logger.Print(AnsiBase + prefix + msg + AnsiReset)
}

// Logging configuration for previews (shared)
var logPreviewLimit int // 0 means unlimited

// SetLogPreviewLimit sets the maximum number of characters to log for
// request/response previews. Set to 0 for unlimited.
func SetLogPreviewLimit(n int) { logPreviewLimit = n }

func previewForLog(s string) string {
    if logPreviewLimit > 0 {
        return trimPreview(s, logPreviewLimit)
    }
    return s
}

