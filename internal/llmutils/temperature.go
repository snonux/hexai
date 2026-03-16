// Package llmutils provides shared utilities for LLM configuration.
// ResolveTemperature centralizes the GPT-5 temperature override logic
// that was previously duplicated across hexaiaction, hexaicli, and lsp.
package llmutils

import "strings"

// isGPT5 returns true when the model name indicates an OpenAI GPT-5 variant.
func isGPT5(provider, model string) bool {
	return provider == "openai" &&
		strings.HasPrefix(strings.ToLower(model), "gpt-5")
}

// ResolveTemperature picks the effective temperature from an optional
// per-surface override (entryTemp) or the global coding temperature
// (codingTemp). For OpenAI GPT-5 models the default coding temperature
// of 0.2 is automatically raised to 1.0, and when no temperature is
// configured at all GPT-5 defaults to 1.0.
//
// Returns (temperature, true) when a value was resolved, or (0, false)
// when no temperature should be sent (let the provider choose).
func ResolveTemperature(provider, model string, entryTemp, codingTemp *float64) (float64, bool) {
	if entryTemp != nil {
		return *entryTemp, true
	}
	if codingTemp != nil {
		temp := *codingTemp
		if isGPT5(provider, model) && temp == 0.2 {
			temp = 1.0
		}
		return temp, true
	}
	if isGPT5(provider, model) {
		return 1.0, true
	}
	return 0, false
}
