package hexaiaction

// Summary: Core types and constants for hexai-action.

type ActionKind string

const (
	ActionSkip        ActionKind = "skip"
	ActionRewrite     ActionKind = "rewrite"
	ActionDiagnostics ActionKind = "diagnostics"
	ActionDocument    ActionKind = "document"
	ActionGoTest      ActionKind = "gotest"
)

// InputParts represents parsed stdin input for actions.
type InputParts struct {
	Selection   string
	Diagnostics []string
}
