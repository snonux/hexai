package hexaiaction

import "context"

func init() {
	registerActionHandler(ActionFixTypos, actionHandlerFunc(handleFixTyposActionRequest))
}

func handleFixTyposActionRequest(ctx context.Context, req actionRequest) (string, error) {
	return handleFixTyposAction(ctx, req.parts, req.cfg, req.client)
}

func handleFixTyposAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer) (string, error) {
	return runWithTimeout(ctx, timeout20s, func(cctx context.Context) (string, error) {
		return runFixTypos(cctx, cfg, client, parts.Selection)
	})
}
