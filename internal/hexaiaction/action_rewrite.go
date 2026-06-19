package hexaiaction

import (
	"context"
	"fmt"
	"io"
	"strings"

	"codeberg.org/snonux/hexai/internal/logging"
)

func init() {
	registerActionHandler(ActionRewrite, actionHandlerFunc(handleRewriteActionRequest))
}

func handleRewriteActionRequest(ctx context.Context, req actionRequest) (string, error) {
	return handleRewriteAction(ctx, req.parts, req.cfg, req.client, req.stderr)
}

func handleRewriteAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer, stderr io.Writer) (string, error) {
	instr, cleaned := ExtractInstruction(parts.Selection)
	if strings.TrimSpace(instr) == "" {
		_, _ = fmt.Fprintln(stderr, logging.AnsiBase+"hexai-tmux-action: no inline instruction found; echoing input"+logging.AnsiReset)
		return parts.Selection, nil
	}
	return runWithTimeout(ctx, timeout20s, func(cctx context.Context) (string, error) {
		return runRewrite(cctx, cfg, client, instr, cleaned)
	})
}
