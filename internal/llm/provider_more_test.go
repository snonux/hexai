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

func TestRegisterProvider_EmptyName(t *testing.T) {
	// An empty (or whitespace-only) name must be rejected with an error
	// instead of panicking, and must not mutate the registry.
	if err := RegisterProvider("   ", func(Config, ProviderKeys) (Client, error) { return nil, nil }); err == nil {
		t.Fatalf("expected error for empty provider name, got nil")
	}
}

func TestRegisterProvider_NilFactory(t *testing.T) {
	// A nil factory is a programming error and must surface as an error.
	if err := RegisterProvider("with-nil-factory", nil); err == nil {
		t.Fatalf("expected error for nil factory, got nil")
	}
}

func TestRegisterProvider_Duplicate(t *testing.T) {
	// "openai" is registered by RegisterAllProviders in TestMain, so a second
	// registration under the same normalized name must return an error.
	if err := RegisterProvider("OpenAI", func(Config, ProviderKeys) (Client, error) { return nil, nil }); err == nil {
		t.Fatalf("expected error for duplicate provider, got nil")
	}
}

func TestRegisterProvider_Success(t *testing.T) {
	// A fresh, valid registration succeeds and is then resolvable.
	name := "test-register-success"
	if err := RegisterProvider(name, func(Config, ProviderKeys) (Client, error) { return nil, nil }); err != nil {
		t.Fatalf("unexpected error registering provider: %v", err)
	}
	if _, ok := lookupProviderFactory(name); !ok {
		t.Fatalf("provider %q not found after successful registration", name)
	}
}

func TestRegisterAllProviders_Idempotent(t *testing.T) {
	// RegisterAllProviders ran in TestMain; calling it again must return the
	// same cached (nil) error without re-registering and tripping a duplicate.
	if err := RegisterAllProviders(); err != nil {
		t.Fatalf("RegisterAllProviders returned error on repeat call: %v", err)
	}
}

func TestNewFromConfig_Success_OpenAI(t *testing.T) {
	// OpenAI success
	oc := Config{Provider: "openai", OpenAIBaseURL: "http://x", OpenAIModel: "gpt"}
	c, err := NewFromConfig(oc, "KEY", "", "", "", "")
	if err != nil || c == nil || c.Name() != "openai" || c.DefaultModel() == "" {
		t.Fatalf("openai new: %v %v", c, err)
	}
}
