package hexaiaction

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// item implements list.Item
type item struct {
	title, desc string
	kind        ActionKind
	hotkey      rune
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {
	list   list.Model
	chosen ActionKind
	done   bool
}

func newModel() model {
	items := []list.Item{
		item{title: "Rewrite selection", desc: "", kind: ActionRewrite, hotkey: 'r'},
		item{title: "Simplify and improve", desc: "", kind: ActionSimplify, hotkey: 'i'},
		item{title: "Document code", desc: "", kind: ActionDocument, hotkey: 'c'},
		item{title: "Generate Go unit test(s)", desc: "", kind: ActionGoTest, hotkey: 't'},
		item{title: "Custom prompt", desc: "", kind: ActionCustom, hotkey: 'p'},
		item{title: "Skip", desc: "", kind: ActionSkip, hotkey: 's'},
	}
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
		// Treat ESC and q as Skip/quit
		m.chosen = ActionSkip
		m.done = true
		return m, tea.Quit
	case "enter":
		if it, ok := m.list.SelectedItem().(item); ok {
			m.chosen = it.kind
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
	case "s", "r", "c", "t", "i", "p":
		items := m.list.Items()
		for i := 0; i < len(items); i++ {
			if it, ok := items[i].(item); ok && strings.ToLower(string(it.hotkey)) == low {
				m.list.Select(i)
				m.chosen = it.kind
				m.done = true
				return m, tea.Quit
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

// RunTUI returns the chosen ActionKind.
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
