package lsp

import (
	"strings"
	"sync"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/llm"
	"github.com/snonux/hexai/internal/llmutils"
	"github.com/snonux/hexai/internal/logging"
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
	provider := llmutils.CanonicalProvider(configuredProvider)
	if client != nil {
		if name := llmutils.CanonicalProvider(client.Name()); name != "" {
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
	provider := llmutils.CanonicalProvider(spec.provider)

	r.clientsMu.RLock()
	baseProvider := r.llmProvider
	baseClient := r.llmClient
	if baseClient != nil && strings.TrimSpace(baseProvider) == "" {
		baseProvider = llmutils.CanonicalProvider(baseClient.Name())
	}
	if provider == "" {
		provider = baseProvider
	}
	requestedModel := strings.TrimSpace(spec.entry.Model)
	if requestedModel == "" {
		requestedModel = strings.TrimSpace(spec.fallbackModel)
	}
	if provider == baseProvider && baseClient != nil && (requestedModel == "" || requestedModel == strings.TrimSpace(baseClient.DefaultModel())) {
		r.clientsMu.RUnlock()
		return baseClient
	}
	cacheKey := provider + ":" + strings.TrimSpace(spec.entry.Model)
	if cached, ok := r.altClients[cacheKey]; ok {
		r.clientsMu.RUnlock()
		return cached
	}
	r.clientsMu.RUnlock()

	modelOverride := strings.TrimSpace(spec.entry.Model)
	if modelOverride == "" {
		modelOverride = strings.TrimSpace(spec.fallbackModel)
	}

	// Acquire write lock before calling build to prevent concurrent goroutines
	// from all passing the read-lock cache-miss check and issuing duplicate
	// build calls for the same provider (TOCTOU race).
	r.clientsMu.Lock()
	defer r.clientsMu.Unlock()

	// Re-check under the write lock; another goroutine may have populated the
	// cache between our RUnlock and this Lock.
	if provider == r.llmProvider && r.llmClient != nil && (requestedModel == "" || requestedModel == strings.TrimSpace(r.llmClient.DefaultModel())) {
		return r.llmClient
	}
	if existing, ok := r.altClients[cacheKey]; ok {
		return existing
	}

	client, err := build(cfg, provider, modelOverride)
	if err != nil {
		logging.Logf("lsp ", "failed to build client for provider=%s: %v", provider, err)
		return nil
	}

	if provider == r.llmProvider {
		if r.llmClient == nil || requestedModel != "" && requestedModel != strings.TrimSpace(r.llmClient.DefaultModel()) {
			if r.altClients == nil {
				r.altClients = make(map[string]llm.Client)
			}
			r.altClients[cacheKey] = client
		}
		if r.llmClient != nil && (requestedModel == "" || requestedModel == strings.TrimSpace(r.llmClient.DefaultModel())) {
			return r.llmClient
		}
		return client
	}
	if r.altClients == nil {
		r.altClients = make(map[string]llm.Client)
	}
	r.altClients[cacheKey] = client
	return client
}
