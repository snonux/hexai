package hexaiaction

import "context"

func init() {
	registerActionHandler(ActionDiagnostics, actionHandlerFunc(handleDiagnosticsActionRequest))
}

func handleDiagnosticsActionRequest(ctx context.Context, req actionRequest) (string, error) {
	return handleDiagnosticsAction(ctx, req.parts, req.cfg, req.client)
}

func handleDiagnosticsAction(ctx context.Context, parts InputParts, cfg actionConfig, client chatDoer) (string, error) {
	return runWithTimeout(ctx, timeout20s, func(cctx context.Context) (string, error) {
		return runDiagnostics(cctx, cfg, client, parts.Diagnostics, parts.Selection)
	})
}
