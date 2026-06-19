package hexaiaction

import "context"

func init() {
	registerActionHandler(ActionDocument, actionHandlerFunc(handleDocumentActionRequest))
}

func handleDocumentActionRequest(ctx context.Context, req actionRequest) (string, error) {
	return handleDocumentAction(ctx, req.parts, req.cfg, req.client)
}

func handleDocumentAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer) (string, error) {
	return runWithTimeout(ctx, timeout20s, func(cctx context.Context) (string, error) {
		return runDocument(cctx, cfg, client, parts.Selection)
	})
}
