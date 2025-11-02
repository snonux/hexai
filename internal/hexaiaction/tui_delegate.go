package hexaiaction

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// oneLineDelegate renders a single compact line per item, no spacing.
type oneLineDelegate struct{}

var (
	hotStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	cursorStyle = lipgloss.NewStyle().Bold(true)
)

func (oneLineDelegate) Height() int                         { return 1 }
func (oneLineDelegate) Spacing() int                        { return 0 }
func (oneLineDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }
func (oneLineDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	title := listItem.FilterValue()
	hk := '?'
	if it, ok := listItem.(item); ok {
		hk = it.hotkey
	}
	hot := hotStyle.Render(fmt.Sprintf(" (%c)", hk))
	cursor := "  "
	if index == m.Index() {
		cursor = cursorStyle.Render("> ")
	}
	_, _ = fmt.Fprintf(w, "%s%s%s", cursor, title, hot)
}
