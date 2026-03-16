// Package hexaiaction provides core types and constants for hexai-tmux-action.
package hexaiaction

type ActionKind string

const (
	ActionSkip        ActionKind = "skip"
	ActionRewrite     ActionKind = "rewrite"
	ActionDiagnostics ActionKind = "diagnostics"
	ActionDocument    ActionKind = "document"
	ActionGoTest      ActionKind = "gotest"
	ActionSimplify    ActionKind = "simplify"
	// ActionCustom represents a configured custom action from the submenu.
	ActionCustom ActionKind = "custom"
	// ActionCustomPrompt is the free-form prompt opened in the editor (hotkey 'p').
	ActionCustomPrompt ActionKind = "custom_prompt"
)

// InputParts represents parsed stdin input for actions.
type InputParts struct {
	Selection   string
	Diagnostics []string
}
