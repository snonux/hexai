package llm

import "testing"

func TestWithOptions_Apply(t *testing.T) {
	o := Options{}
	WithModel("m")(&o)
	WithTemperature(0.7)(&o)
	WithMaxTokens(123)(&o)
	WithStop("END")(&o)
	if o.Model != "m" || o.Temperature != 0.7 || o.MaxTokens != 123 || len(o.Stop) != 1 || o.Stop[0] != "END" {
		t.Fatalf("options not applied correctly: %+v", o)
	}
}

func TestNewFromConfig_Success_OpenAI_And_Copilot(t *testing.T) {
	// OpenAI success
	oc := Config{Provider: "openai", OpenAIBaseURL: "http://x", OpenAIModel: "gpt"}
	c, err := NewFromConfig(oc, "KEY", "")
	if err != nil || c == nil || c.Name() != "openai" || c.DefaultModel() == "" {
		t.Fatalf("openai new: %v %v", c, err)
	}
	// Copilot success
	cc := Config{Provider: "copilot", CopilotBaseURL: "http://x", CopilotModel: "gpt-4o-mini"}
	c2, err := NewFromConfig(cc, "", "KEY")
	if err != nil || c2 == nil || c2.Name() != "copilot" || c2.DefaultModel() == "" {
		t.Fatalf("copilot new: %v %v", c2, err)
	}
}
