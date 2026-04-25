package hexaiaction

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

// item implements list.Item; custom is non-nil when the entry is a custom
// action placed directly in the main menu (not in the submenu).
type item struct {
	title, desc string
	kind        ActionKind
	hotkey      rune
	custom      *appconfig.CustomAction
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {
	list         list.Model
	chosen       ActionKind
	chosenCustom *appconfig.CustomAction
	done         bool
}

func newModel() model {
	items := []list.Item{
		item{title: "Rewrite selection", desc: "", kind: ActionRewrite, hotkey: 'r'},
		item{title: "Simplify and improve", desc: "", kind: ActionSimplify, hotkey: 'i'},
		item{title: "Document code", desc: "", kind: ActionDocument, hotkey: 'c'},
		item{title: "Generate Go unit test(s)", desc: "", kind: ActionGoTest, hotkey: 't'},
		item{title: "Fix typos and improve grammar and clarity", desc: "", kind: ActionFixTypos, hotkey: 'f'},
		item{title: "Custom prompt", desc: "", kind: ActionCustomPrompt, hotkey: 'p'},
		item{title: "Skip", desc: "", kind: ActionSkip, hotkey: 's'},
	}
	return newListModel(items)
}

func newListModel(items []list.Item) model {
	l := list.New(items, oneLineDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	return model{list: l}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return handleKey(m, msg)
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func handleKey(m model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	raw := msg.String()
	low := strings.ToLower(raw)
	switch low {
	case "esc", "q":
		m.chosen = ActionSkip
		m.done = true
		return m, tea.Quit
	case "enter":
		if it, ok := m.list.SelectedItem().(item); ok {
			m.chosen = it.kind
			m.chosenCustom = it.custom
			m.done = true
			return m, tea.Quit
		}
	case "j", "down":
		m.list.CursorDown()
	case "k", "up":
		m.list.CursorUp()
	case "g", "home":
		m.list.Select(0)
	case "end":
		if n := len(m.list.Items()); n > 0 {
			m.list.Select(n - 1)
		}
	default:
		// Dynamic hotkey dispatch: any single-rune key matching an item's hotkey.
		if rr := []rune(low); len(rr) == 1 {
			items := m.list.Items()
			for i, li := range items {
				if it, ok := li.(item); ok && it.hotkey != 0 && string(it.hotkey) == low {
					m.list.Select(i)
					m.chosen = it.kind
					m.chosenCustom = it.custom
					m.done = true
					return m, tea.Quit
				}
			}
		}
	}
	if raw == "G" { // Shift+G jumps to end
		if n := len(m.list.Items()); n > 0 {
			m.list.Select(n - 1)
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.done {
		return ""
	}
	return m.list.View()
}

// RunTUI returns the chosen ActionKind from the default hardcoded menu.
func RunTUI() (ActionKind, error) {
	p := tea.NewProgram(newModel())
	md, err := p.Run()
	if err != nil {
		return ActionSkip, err
	}
	if m, ok := md.(model); ok {
		if m.chosen == "" {
			return ActionSkip, nil
		}
		return m.chosen, nil
	}
	return ActionSkip, fmt.Errorf("unexpected model type")
}

// RunTUIFromConfig builds the menu from entries and returns the chosen action.
// Custom entries are resolved by ID against customs. Falls back to ActionSkip
// if the program returns an unexpected model type.
func RunTUIFromConfig(entries []appconfig.TmuxActionMenuEntry, customs []appconfig.CustomAction) (ActionKind, *appconfig.CustomAction, error) {
	m := newModelFromMenuEntries(entries, customs)
	p := teaNewProgram(m)
	md, err := p.Run()
	if err != nil {
		return ActionSkip, nil, err
	}
	if mm, ok := md.(model); ok {
		if mm.chosen == "" {
			return ActionSkip, nil, nil
		}
		return mm.chosen, mm.chosenCustom, nil
	}
	return ActionSkip, nil, fmt.Errorf("unexpected model type")
}

// newModelFromMenuEntries builds a model from config-driven menu entries.
// Custom entries are resolved by ID against the provided customs slice;
// entries with unknown IDs are silently skipped.
func newModelFromMenuEntries(entries []appconfig.TmuxActionMenuEntry, customs []appconfig.CustomAction) model {
	byID := make(map[string]*appconfig.CustomAction, len(customs))
	for i := range customs {
		id := strings.ToLower(strings.TrimSpace(customs[i].ID))
		if id != "" {
			byID[id] = &customs[i]
		}
	}
	items := make([]list.Item, 0, len(entries))
	for _, e := range entries {
		kind := ActionKind(strings.TrimSpace(e.Kind))
		var ca *appconfig.CustomAction
		if kind == ActionCustom {
			ca = byID[strings.ToLower(strings.TrimSpace(e.CustomID))]
			if ca == nil {
				continue
			}
		}
		title := strings.TrimSpace(e.Title)
		if title == "" {
			title = defaultTitleForKind(kind, ca)
		}
		hotkey := hotkeyFromString(e.Hotkey)
		if hotkey == 0 {
			hotkey = defaultHotkeyForKind(kind, ca)
		}
		items = append(items, item{title: title, kind: kind, hotkey: hotkey, custom: ca})
	}
	return newListModel(items)
}

// defaultTitleForKind returns the built-in display title for a given action kind.
func defaultTitleForKind(kind ActionKind, ca *appconfig.CustomAction) string {
	if kind == ActionCustom && ca != nil {
		return ca.Title
	}
	switch kind {
	case ActionRewrite:
		return "Rewrite selection"
	case ActionSimplify:
		return "Simplify and improve"
	case ActionDocument:
		return "Document code"
	case ActionGoTest:
		return "Generate Go unit test(s)"
	case ActionFixTypos:
		return "Fix typos and improve grammar and clarity"
	case ActionCustomPrompt:
		return "Custom prompt"
	case ActionSkip:
		return "Skip"
	}
	return string(kind)
}

// defaultHotkeyForKind returns the built-in hotkey rune for a given action kind.
func defaultHotkeyForKind(kind ActionKind, ca *appconfig.CustomAction) rune {
	if kind == ActionCustom && ca != nil {
		return hotkeyFromString(ca.Hotkey)
	}
	switch kind {
	case ActionRewrite:
		return 'r'
	case ActionSimplify:
		return 'i'
	case ActionDocument:
		return 'c'
	case ActionGoTest:
		return 't'
	case ActionFixTypos:
		return 'f'
	case ActionCustomPrompt:
		return 'p'
	case ActionSkip:
		return 's'
	}
	return 0
}

// hotkeyFromString decodes the first rune from s, returning 0 on empty or error.
func hotkeyFromString(s string) rune {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return 0
	}
	return r
}
