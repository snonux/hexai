package hexaicli

import (
	"context"
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

// responseCache carries the injectable dependencies for the on-disk CLI
// response cache. The only dependency is the clock used to stamp entries and
// decide expiry. Production code uses defaultResponseCache (backed by
// time.Now); tests construct a responseCache with a fake clock to exercise TTL
// expiry without sleeping.
type responseCache struct {
	now func() time.Time
}

// defaultResponseCache is the production cache used by the package-level
// lookup/store wrappers. It reads the real wall clock.
var defaultResponseCache = responseCache{now: time.Now}

// cacheNowContextKey carries an injected clock through the request context so
// the cache TTL logic can be driven deterministically (e.g. in tests) without
// mutating package state.
type cacheNowContextKey struct{}

// withCLIResponseCacheNow returns a context carrying now as the clock the CLI
// response cache should use for stamping and expiring entries.
func withCLIResponseCacheNow(ctx context.Context, now func() time.Time) context.Context {
	return context.WithValue(ctx, cacheNowContextKey{}, now)
}

// responseCacheFromContext builds a responseCache using the clock injected via
// withCLIResponseCacheNow, falling back to the real wall clock.
func responseCacheFromContext(ctx context.Context) responseCache {
	if now, ok := ctx.Value(cacheNowContextKey{}).(func() time.Time); ok && now != nil {
		return responseCache{now: now}
	}
	return defaultResponseCache
}

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

// lookupCLIResponseCache reads a cached response using the clock injected into
// ctx (defaulting to the real wall clock).
func lookupCLIResponseCache(ctx context.Context, key cliResponseCacheKey) (string, time.Duration, bool) {
	return responseCacheFromContext(ctx).lookup(key)
}

// storeCLIResponseCache writes a cached response using the clock injected into
// ctx (defaulting to the real wall clock).
func storeCLIResponseCache(ctx context.Context, key cliResponseCacheKey, output string) {
	responseCacheFromContext(ctx).store(key, output)
}

// lookup returns the cached output for key, its age, and whether it is a valid
// (non-expired) hit. Expired entries are removed.
func (c responseCache) lookup(key cliResponseCacheKey) (string, time.Duration, bool) {
	path, ok := cliResponseCachePath(key)
	if !ok {
		return "", 0, false
	}
	entry, ok := loadCLIResponseCacheEntry(path)
	if !ok {
		return "", 0, false
	}
	age := c.now().Sub(entry.CreatedAt)
	if age > cliResponseCacheTTL {
		_ = os.Remove(path)
		return "", 0, false
	}
	return entry.Output, age, true
}

// store persists output for key, stamping it with the injected clock.
func (c responseCache) store(key cliResponseCacheKey, output string) {
	path, ok := cliResponseCachePath(key)
	if !ok {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	entry := cliResponseCacheEntry{CreatedAt: c.now().UTC(), Output: output}
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
