package lsp

import (
	"strings"
	"sync"

	"codeberg.org/snonux/hexai/internal/appconfig"
	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/logging"
)

type llmClientBuilder func(appconfig.App, string, string) (llm.Client, error)

type llmClientRegistry struct {
	clientsMu   sync.RWMutex
	llmClient   llm.Client
	llmProvider string
	altClients  map[string]llm.Client
}

func newLLMClientRegistry() llmClientRegistry {
	return llmClientRegistry{
		altClients: make(map[string]llm.Client),
	}
}

func (r *llmClientRegistry) applyOptions(client llm.Client, configuredProvider string) {
	provider := canonicalProvider(configuredProvider)
	if client != nil {
		if name := canonicalProvider(client.Name()); name != "" {
			provider = name
		}
	}
	r.clientsMu.Lock()
	defer r.clientsMu.Unlock()
	r.llmClient = client
	r.llmProvider = provider
	r.altClients = make(map[string]llm.Client)
}

func (r *llmClientRegistry) current() llm.Client {
	r.clientsMu.RLock()
	defer r.clientsMu.RUnlock()
	return r.llmClient
}

func (r *llmClientRegistry) clientFor(spec requestSpec, cfg appconfig.App, build llmClientBuilder) llm.Client {
	provider := canonicalProvider(spec.provider)

	r.clientsMu.RLock()
	baseProvider := r.llmProvider
	baseClient := r.llmClient
	if baseClient != nil && strings.TrimSpace(baseProvider) == "" {
		baseProvider = canonicalProvider(baseClient.Name())
	}
	if provider == "" {
		provider = baseProvider
	}
	if provider == baseProvider && baseClient != nil {
		r.clientsMu.RUnlock()
		return baseClient
	}
	if cached, ok := r.altClients[provider]; ok {
		r.clientsMu.RUnlock()
		return cached
	}
	r.clientsMu.RUnlock()

	modelOverride := strings.TrimSpace(spec.entry.Model)
	if modelOverride == "" {
		modelOverride = strings.TrimSpace(spec.fallbackModel)
	}
	client, err := build(cfg, provider, modelOverride)
	if err != nil {
		logging.Logf("lsp ", "failed to build client for provider=%s: %v", provider, err)
		if baseClient != nil {
			return baseClient
		}
		return nil
	}

	r.clientsMu.Lock()
	defer r.clientsMu.Unlock()
	if provider == r.llmProvider {
		if r.llmClient == nil {
			r.llmClient = client
			r.llmProvider = provider
		}
		return r.llmClient
	}
	if existing, ok := r.altClients[provider]; ok {
		return existing
	}
	if r.altClients == nil {
		r.altClients = make(map[string]llm.Client)
	}
	r.altClients[provider] = client
	return client
}
