package hexaiaction

import (
	"context"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

func init() {
	registerActionHandler(ActionCustom, actionHandlerFunc(handleCustomActionRequest))
}

func handleCustomActionRequest(ctx context.Context, req actionRequest) (string, error) {
	return handleCustomAction(ctx, req.parts, req.cfg, req.client, req.selectedCustom)
}

func handleCustomAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer, selectedCustom *appconfig.CustomAction) (string, error) {
	if selectedCustom == nil {
		return parts.Selection, nil
	}
	custom := *selectedCustom
	return runWithTimeout(ctx, timeout20s, func(cctx context.Context) (string, error) {
		return runCustom(cctx, cfg, client, custom, parts)
	})
}
