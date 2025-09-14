package hexaiaction

import (
	"context"
	"strings"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

func TestExecuteAction_CustomConfigured_Instruction(t *testing.T) {
	cfg := appconfig.Load(nil)
	parts := InputParts{Selection: "code"}
	selectedCustom = &appconfig.CustomAction{ID: "x", Title: "X", Instruction: "Do it"}
	out, err := executeAction(context.Background(), ActionCustom, parts, cfg, fakeDoer{"OK"}, nil)
	if err != nil || strings.TrimSpace(out) != "OK" {
		t.Fatalf("custom-instruction failed: %q %v", out, err)
	}
}

func TestExecuteAction_CustomConfigured_User(t *testing.T) {
	cfg := appconfig.Load(nil)
	parts := InputParts{Selection: "sel"}
	selectedCustom = &appconfig.CustomAction{ID: "y", Title: "Y", User: "Apply to: {{selection}}"}
	out, err := executeAction(context.Background(), ActionCustom, parts, cfg, fakeDoer{"OK2"}, nil)
	if err != nil || strings.TrimSpace(out) != "OK2" {
		t.Fatalf("custom-user failed: %q %v", out, err)
	}
}
