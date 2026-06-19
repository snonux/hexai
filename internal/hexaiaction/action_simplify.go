package hexaiaction

import "context"

func init() {
	registerActionHandler(ActionSimplify, actionHandlerFunc(handleSimplifyActionRequest))
}

func handleSimplifyActionRequest(ctx context.Context, req actionRequest) (string, error) {
	return handleSimplifyAction(ctx, req.parts, req.cfg, req.client)
}

func handleSimplifyAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer) (string, error) {
	return runWithTimeout(ctx, timeout20s, func(cctx context.Context) (string, error) {
		return runSimplify(cctx, cfg, client, parts.Selection)
	})
}
