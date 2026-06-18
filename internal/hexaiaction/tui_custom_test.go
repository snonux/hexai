package hexaiaction

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

type fakeProg struct {
	m     model
	onRun func(*model)
}

func (f fakeProg) Run() (tea.Model, error) {
	if f.onRun != nil {
		f.onRun(&f.m)
	}
	return f.m, nil
}

func TestRunTUIWithCustom_SubmenuAndHotkeys(t *testing.T) {
	calls := 0
	r := tuiRunner{newProgram: func(m model) teaProgram {
		calls++
		if calls == 1 {
			// Main menu should have "Custom actions…" with configured hotkey
			items := m.list.Items()
			if len(items) == 0 {
				t.Fatalf("main menu items empty")
			}
			last := items[len(items)-1].(item)
			if last.title != "Custom actions…" || string(last.hotkey) != "z" {
				t.Fatalf("custom menu entry wrong: title=%q hotkey=%q", last.title, string(last.hotkey))
			}
			return fakeProg{m: m, onRun: func(mm *model) { mm.chosen = ActionCustom }}
		}
		if calls == 2 {
			// Submenu should list our custom actions with mapped hotkeys
			items := m.list.Items()
			if len(items) != 2 {
				t.Fatalf("submenu expected 2 items, got %d", len(items))
			}
			a := items[0].(item)
			b := items[1].(item)
			if a.title != "A" || string(a.hotkey) != "x" {
				t.Fatalf("first submenu item wrong: %q/%q", a.title, string(a.hotkey))
			}
			if b.title != "B" || string(b.hotkey) != "y" {
				t.Fatalf("second submenu item wrong: %q/%q", b.title, string(b.hotkey))
			}
			// Simulate selecting first item
			return fakeProg{m: m, onRun: func(mm *model) { mm.list.Select(0) }}
		}
		return fakeProg{m: m}
	}}

	customs := []appconfig.CustomAction{
		{ID: "a", Title: "A", Hotkey: "x", Instruction: "do"},
		{ID: "b", Title: "B", Hotkey: "y", Instruction: "do2"},
	}
	kind, selected, err := r.RunTUIWithCustom(customs, "z")
	if err != nil {
		t.Fatalf("RunTUIWithCustom error: %v", err)
	}
	if kind != ActionCustom {
		t.Fatalf("expected ActionCustom, got %s", kind)
	}
	if selected == nil || selected.ID != "a" {
		t.Fatalf("expected selected custom a, got %+v", selected)
	}
}
