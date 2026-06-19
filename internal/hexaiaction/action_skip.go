package hexaiaction

import "context"

func init() {
	registerActionHandler(ActionSkip, actionHandlerFunc(handleSkipAction))
}

func handleSkipAction(_ context.Context, req actionRequest) (string, error) {
	return req.parts.Selection, nil
}
