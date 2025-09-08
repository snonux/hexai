package llmutils

import (
	"os"
	"testing"

	"codeberg.org/snonux/hexai/internal/appconfig"
)

func TestNewClientFromApp_Ollama(t *testing.T) {
	cfg := appconfig.App{Provider: "ollama"}
	c, err := NewClientFromApp(cfg)
	if err != nil || c == nil {
		t.Fatalf("ollama client failed: %v %v", c, err)
	}
}

func TestNewClientFromApp_OpenAI_WithKey(t *testing.T) {
	t.Setenv("HEXAI_OPENAI_API_KEY", "test-key")
	cfg := appconfig.App{Provider: "openai"}
	c, err := NewClientFromApp(cfg)
	if err != nil || c == nil {
		t.Fatalf("openai client failed: %v %v", c, err)
	}
	// ensure env override precedence
	_ = os.Unsetenv("OPENAI_API_KEY")
}
