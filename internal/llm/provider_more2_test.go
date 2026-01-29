package llm

import "testing"

func TestNewFromConfig_Copilot(t *testing.T) {
	t.Setenv("COPILOT_API_KEY", "x")
	cfg := Config{Provider: "copilot", CopilotModel: "small"}
	c, err := NewFromConfig(cfg, "", "", "x", "")
	if err != nil || c == nil {
		t.Fatalf("copilot provider failed: %v %v", c, err)
	}
}
