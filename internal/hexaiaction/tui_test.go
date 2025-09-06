package hexaiaction

import (
    "testing"

    tea "github.com/charmbracelet/bubbletea"
)

func TestHandleKey_EscSkips(t *testing.T) {
    m := newModel()
    nm, _ := handleKey(m, tea.KeyMsg{Type: tea.KeyEsc})
    got, ok := nm.(model)
    if !ok || !got.done || got.chosen != ActionSkip {
        t.Fatalf("esc should skip: ok=%v done=%v chosen=%v", ok, got.done, got.chosen)
    }
}

func TestHandleKey_QuickHotkey(t *testing.T) {
    m := newModel()
    nm, _ := handleKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
    got := nm.(model)
    if !got.done || got.chosen != ActionRewrite {
        t.Fatalf("r should choose rewrite: done=%v chosen=%v", got.done, got.chosen)
    }
}

func TestHandleKey_JumpEndWithG(t *testing.T) {
    m := newModel()
    // raw 'G' rune should jump to end (special cased)
    nm, _ := handleKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
    got := nm.(model)
    if idx := got.list.Index(); idx != len(got.list.Items())-1 {
        t.Fatalf("G should jump to end, index=%d", idx)
    }
}

