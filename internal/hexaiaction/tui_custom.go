package hexaiaction

import (
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

// RunTUIWithCustom shows the main menu plus a configurable "Custom actions…" item.
// If the user selects that item, it shows a submenu listing user-defined custom actions.
func RunTUIWithCustom(customs []appconfig.CustomAction, menuHotkey string) (ActionKind, *appconfig.CustomAction, error) {
	// When no customs, fall back to default menu
	if len(customs) == 0 {
		kind, err := RunTUI()
		return kind, nil, err
	}
	// Build main menu with an extra entry
	hk := 'a'
	if r, _ := utf8.DecodeRuneInString(menuHotkey); r != utf8.RuneError && r != 0 {
		hk = r
	}
	// Create a model with default items plus Custom actions…
	m := newModel()
	items := m.list.Items()
	items = append(items, item{title: "Custom actions…", desc: "", kind: ActionCustom, hotkey: hk})
	m.list.SetItems(items)
	// Run main menu
	p := teaNewProgram(m)
	md, err := p.Run()
	if err != nil {
		return ActionSkip, nil, err
	}
	if mm, ok := md.(model); ok {
		// If user chose built-in items (including Custom prompt), return immediately.
		if mm.chosen != ActionCustom {
			return mm.chosen, nil, nil
		}
	}
	// Custom submenu: list each action; selection maps to ActionCustom and returns that action.
	sub := newModel()
	subItems := make([]list.Item, 0, len(customs))
	for _, ca := range customs {
		r := rune(0)
		if rr, _ := utf8.DecodeRuneInString(ca.Hotkey); rr != utf8.RuneError && rr != 0 {
			r = rr
		}
		subItems = append(subItems, item{title: ca.Title, desc: "", kind: ActionCustom, hotkey: r})
	}
	sub.list.SetItems(subItems)
	sp := teaNewProgram(sub)
	smd, err := sp.Run()
	if err != nil {
		return ActionSkip, nil, err
	}
	if sm, ok := smd.(model); ok {
		if it, ok := sm.list.SelectedItem().(item); ok {
			// Map by title
			for i := range customs {
				if customs[i].Title == it.title {
					c := customs[i]
					return ActionCustom, &c, nil
				}
			}
		}
	}
	return ActionSkip, nil, nil
}

// teaNewProgram is a tiny seam for tests to stub bubbletea program creation.
var teaNewProgram = func(m model) teaProgram { return tea.NewProgram(m) }

// teaProgram is the subset of bubbletea.Program we need; enables testing seam.
type teaProgram interface{ Run() (tea.Model, error) }
