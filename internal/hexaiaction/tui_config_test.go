package hexaiaction

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

func TestDefaultTitleForKind(t *testing.T) {
	cases := []struct {
		kind ActionKind
		want string
	}{
		{ActionRewrite, "Rewrite selection"},
		{ActionSimplify, "Simplify and improve"},
		{ActionDocument, "Document code"},
		{ActionGoTest, "Generate Go unit test(s)"},
		{ActionFixTypos, "Fix typos and improve grammar and clarity"},
		{ActionCustomPrompt, "Custom prompt"},
		{ActionSkip, "Skip"},
		{"unknown", "unknown"},
	}
	for _, tc := range cases {
		if got := defaultTitleForKind(tc.kind, nil); got != tc.want {
			t.Errorf("defaultTitleForKind(%q) = %q, want %q", tc.kind, got, tc.want)
		}
	}
	ca := appconfig.CustomAction{Title: "My Action"}
	if got := defaultTitleForKind(ActionCustom, &ca); got != "My Action" {
		t.Errorf("custom title: got %q", got)
	}
}

func TestDefaultHotkeyForKind(t *testing.T) {
	cases := []struct {
		kind ActionKind
		want rune
	}{
		{ActionRewrite, 'r'},
		{ActionSimplify, 'i'},
		{ActionDocument, 'c'},
		{ActionGoTest, 't'},
		{ActionFixTypos, 'f'},
		{ActionCustomPrompt, 'p'},
		{ActionSkip, 's'},
		{"unknown", 0},
	}
	for _, tc := range cases {
		if got := defaultHotkeyForKind(tc.kind, nil); got != tc.want {
			t.Errorf("defaultHotkeyForKind(%q) = %q, want %q", tc.kind, got, tc.want)
		}
	}
	ca := appconfig.CustomAction{Hotkey: "x"}
	if got := defaultHotkeyForKind(ActionCustom, &ca); got != 'x' {
		t.Errorf("custom hotkey: got %q", got)
	}
}

func TestHotkeyFromString(t *testing.T) {
	if hotkeyFromString("") != 0 {
		t.Fatal("empty should be 0")
	}
	if hotkeyFromString("  ") != 0 {
		t.Fatal("blank should be 0")
	}
	if hotkeyFromString("r") != 'r' {
		t.Fatal("r should be r")
	}
	if hotkeyFromString("rX") != 'r' {
		t.Fatal("first rune of rX should be r")
	}
}

func TestNewModelFromMenuEntries_BasicOrder(t *testing.T) {
	entries := []appconfig.TmuxActionMenuEntry{
		{Kind: "skip", Hotkey: "s"},
		{Kind: "rewrite", Title: "Redo", Hotkey: "w"},
	}
	m := newModelFromMenuEntries(entries, nil)
	items := m.list.Items()
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	first := items[0].(item)
	if first.kind != ActionSkip || first.hotkey != 's' {
		t.Errorf("first item wrong: %+v", first)
	}
	second := items[1].(item)
	if second.kind != ActionRewrite || second.title != "Redo" || second.hotkey != 'w' {
		t.Errorf("second item wrong: %+v", second)
	}
}

func TestNewModelFromMenuEntries_CustomAction(t *testing.T) {
	customs := []appconfig.CustomAction{
		{ID: "my-action", Title: "My Action", Hotkey: "m", Instruction: "do it"},
	}
	entries := []appconfig.TmuxActionMenuEntry{
		{Kind: "custom", CustomID: "my-action"},
		{Kind: "custom", CustomID: "missing-id"}, // should be skipped
	}
	m := newModelFromMenuEntries(entries, customs)
	items := m.list.Items()
	if len(items) != 1 {
		t.Fatalf("expected 1 item (missing ID skipped), got %d", len(items))
	}
	it := items[0].(item)
	if it.title != "My Action" || it.hotkey != 'm' || it.custom == nil {
		t.Errorf("custom item wrong: %+v", it)
	}
}

func TestNewModelFromMenuEntries_DefaultsApplied(t *testing.T) {
	entries := []appconfig.TmuxActionMenuEntry{
		{Kind: "simplify"}, // no title/hotkey → use defaults
	}
	m := newModelFromMenuEntries(entries, nil)
	it := m.list.Items()[0].(item)
	if it.title != "Simplify and improve" || it.hotkey != 'i' {
		t.Errorf("defaults not applied: %+v", it)
	}
}

func TestHandleKey_DynamicHotkey(t *testing.T) {
	entries := []appconfig.TmuxActionMenuEntry{
		{Kind: "simplify", Hotkey: "z"}, // non-default hotkey
		{Kind: "skip", Hotkey: "s"},
	}
	m := newModelFromMenuEntries(entries, nil)
	nm, _ := handleKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	got := nm.(model)
	if !got.done || got.chosen != ActionSimplify {
		t.Fatalf("z should choose simplify: done=%v chosen=%v", got.done, got.chosen)
	}
}

func TestHandleKey_ChosenCustomIsSet(t *testing.T) {
	ca := appconfig.CustomAction{ID: "a", Title: "A", Hotkey: "a", Instruction: "x"}
	customs := []appconfig.CustomAction{ca}
	entries := []appconfig.TmuxActionMenuEntry{
		{Kind: "custom", CustomID: "a"},
	}
	m := newModelFromMenuEntries(entries, customs)
	nm, _ := handleKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := nm.(model)
	if !got.done || got.chosen != ActionCustom || got.chosenCustom == nil {
		t.Fatalf("chosenCustom should be set: done=%v chosen=%v custom=%v", got.done, got.chosen, got.chosenCustom)
	}
	if got.chosenCustom.ID != "a" {
		t.Fatalf("wrong custom: %+v", got.chosenCustom)
	}
}

func TestRunTUIFromConfig_ViaTmuxActionSeam(t *testing.T) {
	r := tuiRunner{newProgram: func(m model) teaProgram {
		return fakeProg{m: m, onRun: func(mm *model) {
			mm.chosen = ActionSkip
		}}
	}}

	entries := []appconfig.TmuxActionMenuEntry{
		{Kind: "skip", Hotkey: "s"},
	}
	kind, custom, err := r.RunTUIFromConfig(entries, nil)
	if err != nil {
		t.Fatalf("RunTUIFromConfig: %v", err)
	}
	if kind != ActionSkip || custom != nil {
		t.Fatalf("expected ActionSkip/nil, got %v/%v", kind, custom)
	}
}
