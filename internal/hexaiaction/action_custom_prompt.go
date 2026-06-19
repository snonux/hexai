package hexaiaction

import (
	"context"
	"fmt"
	"io"
	"strings"

	"codeberg.org/snonux/hexai/internal/logging"
)

func init() {
	registerActionHandler(ActionCustomPrompt, actionHandlerFunc(handleCustomPromptActionRequest))
}

func handleCustomPromptActionRequest(ctx context.Context, req actionRequest) (string, error) {
	return handleCustomPromptAction(ctx, req.parts, req.cfg, req.client, req.stderr)
}

func handleCustomPromptAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer, stderr io.Writer) (string, error) {
	prompt, err := actionEditorFromContext(ctx)(ctx, nil)
	if err != nil || strings.TrimSpace(prompt) == "" {
		_, _ = fmt.Fprintln(stderr, logging.AnsiBase+"hexai-tmux-action: custom prompt canceled or empty; echoing input"+logging.AnsiReset)
		return parts.Selection, nil
	}
	return runWithTimeout(ctx, timeout20s, func(cctx context.Context) (string, error) {
		return runRewrite(cctx, cfg, client, prompt, parts.Selection)
	})
}
