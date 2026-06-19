package hexaiaction

import "context"

func init() {
	registerActionHandler(ActionGoTest, actionHandlerFunc(handleGoTestActionRequest))
}

func handleGoTestActionRequest(ctx context.Context, req actionRequest) (string, error) {
	return handleGoTestAction(ctx, req.parts, req.cfg, req.client)
}

func handleGoTestAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer) (string, error) {
	return runWithTimeout(ctx, timeout18s, func(cctx context.Context) (string, error) {
		return runGoTest(cctx, cfg, client, parts.Selection)
	})
}
