package hexaicli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/snonux/hexai/internal/llm"
	"codeberg.org/snonux/hexai/internal/stats"
)

const cliResponseCacheTTL = 24 * time.Hour

var nowCLIResponseCache = time.Now

type cliResponseCacheKey struct {
	Provider    string        `json:"provider"`
	Model       string        `json:"model"`
	Messages    []llm.Message `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

type cliResponseCacheEntry struct {
	CreatedAt time.Time `json:"created_at"`
	Output    string    `json:"output"`
}

func newCLIResponseCacheKey(provider, model string, req requestArgs, msgs []llm.Message) cliResponseCacheKey {
	return cliResponseCacheKey{
		Provider:    provider,
		Model:       model,
		Messages:    cloneCLIMessages(msgs),
		MaxTokens:   req.maxTokens,
		Temperature: cloneCLITemperature(req.temperature),
	}
}

func lookupCLIResponseCache(key cliResponseCacheKey) (string, time.Duration, bool) {
	path, ok := cliResponseCachePath(key)
	if !ok {
		return "", 0, false
	}
	entry, ok := loadCLIResponseCacheEntry(path)
	if !ok {
		return "", 0, false
	}
	age := nowCLIResponseCache().Sub(entry.CreatedAt)
	if age > cliResponseCacheTTL {
		_ = os.Remove(path)
		return "", 0, false
	}
	return entry.Output, age, true
}

func storeCLIResponseCache(key cliResponseCacheKey, output string) {
	path, ok := cliResponseCachePath(key)
	if !ok {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	entry := cliResponseCacheEntry{CreatedAt: nowCLIResponseCache().UTC(), Output: output}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

func cliResponseCachePath(key cliResponseCacheKey) (string, bool) {
	dir, err := stats.CacheDir()
	if err != nil {
		return "", false
	}
	fingerprint, ok := cliResponseCacheFingerprint(key)
	if !ok {
		return "", false
	}
	return filepath.Join(dir, "cli-responses", fingerprint+".json"), true
}

func cliResponseCacheFingerprint(key cliResponseCacheKey) (string, bool) {
	data, err := json.Marshal(key)
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), true
}

func loadCLIResponseCacheEntry(path string) (cliResponseCacheEntry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cliResponseCacheEntry{}, false
	}
	var entry cliResponseCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		_ = os.Remove(path)
		return cliResponseCacheEntry{}, false
	}
	return entry, true
}

func cloneCLIMessages(msgs []llm.Message) []llm.Message {
	out := make([]llm.Message, len(msgs))
	copy(out, msgs)
	return out
}

func cloneCLITemperature(temp *float64) *float64 {
	if temp == nil {
		return nil
	}
	value := *temp
	return &value
}
