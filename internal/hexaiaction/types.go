package hexaiaction

// Summary: Core types and constants for hexai-tmux-action.

type ActionKind string

const (
	ActionSkip        ActionKind = "skip"
	ActionRewrite     ActionKind = "rewrite"
	ActionDiagnostics ActionKind = "diagnostics"
	ActionDocument    ActionKind = "document"
	ActionGoTest      ActionKind = "gotest"
	ActionSimplify    ActionKind = "simplify"
	ActionCustom      ActionKind = "custom"
)

// InputParts represents parsed stdin input for actions.
type InputParts struct {
	Selection   string
	Diagnostics []string
}
