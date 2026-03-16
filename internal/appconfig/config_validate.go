package appconfig

import (
	"fmt"
	"strings"
)

// Validate checks custom actions and tmux settings for duplicates and consistency.
func (a *App) Validate() error {
	if err := validateCustomActions(a.CustomActions); err != nil {
		return err
	}
	return validateTmuxCustomMenuHotkey(a.TmuxCustomMenuHotkey)
}

func validateCustomActions(actions []CustomAction) error {
	seenID := make(map[string]struct{})
	seenHK := make(map[string]struct{})
	for _, action := range actions {
		if err := validateCustomAction(action, seenID, seenHK); err != nil {
			return err
		}
	}
	return nil
}

func validateCustomAction(action CustomAction, seenID, seenHK map[string]struct{}) error {
	id := strings.ToLower(strings.TrimSpace(action.ID))
	if id == "" {
		return fmt.Errorf("config: custom action missing required field id")
	}
	if _, exists := seenID[id]; exists {
		return fmt.Errorf("config: duplicate custom action id: %s", action.ID)
	}
	seenID[id] = struct{}{}

	if strings.TrimSpace(action.Title) == "" {
		return fmt.Errorf("config: custom action %s missing required field title", action.ID)
	}
	if err := validateCustomScope(action); err != nil {
		return err
	}
	if err := validateCustomInstructionOrUser(action); err != nil {
		return err
	}
	return validateCustomHotkey(action, seenHK)
}

func validateCustomScope(action CustomAction) error {
	scope := strings.TrimSpace(action.Scope)
	if scope == "" || scope == "selection" || scope == "diagnostics" {
		return nil
	}
	return fmt.Errorf("config: custom action %s has invalid scope: %s", action.ID, action.Scope)
}

func validateCustomInstructionOrUser(action CustomAction) error {
	hasInstr := strings.TrimSpace(action.Instruction) != ""
	hasUser := strings.TrimSpace(action.User) != ""
	if hasInstr && hasUser {
		return fmt.Errorf("config: custom action %s must set either instruction or user, not both", action.ID)
	}
	if !hasInstr && !hasUser {
		return fmt.Errorf("config: custom action %s requires instruction or user", action.ID)
	}
	return nil
}

func validateCustomHotkey(action CustomAction, seenHK map[string]struct{}) error {
	hk := strings.TrimSpace(action.Hotkey)
	if hk == "" {
		return nil
	}
	if len([]rune(hk)) != 1 {
		return fmt.Errorf("config: custom action %s hotkey must be a single character", action.ID)
	}
	lower := strings.ToLower(hk)
	if _, exists := seenHK[lower]; exists {
		return fmt.Errorf("config: duplicate custom action hotkey: %s", hk)
	}
	seenHK[lower] = struct{}{}
	return nil
}

func validateTmuxCustomMenuHotkey(hotkey string) error {
	hk := strings.TrimSpace(hotkey)
	if hk == "" {
		return nil
	}
	if len([]rune(hk)) != 1 {
		return fmt.Errorf("config: invalid tmux.custom_menu_hotkey: %s", hk)
	}
	switch strings.ToLower(hk) {
	case "r", "i", "c", "t", "p", "s":
		return fmt.Errorf("config: invalid tmux.custom_menu_hotkey: %s (clashes with built-in)", hk)
	}
	return nil
}
