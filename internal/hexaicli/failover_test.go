package hexaicli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/llm"
	"github.com/snonux/hexai/internal/llmutils"
)

type cliFailoverClient struct {
	name string
	err  error
	text string
}

func (c *cliFailoverClient) Chat(context.Context, []llm.Message, ...llm.RequestOption) (string, error) {
	return c.text, c.err
}

func (c *cliFailoverClient) Name() string         { return c.name }
func (c *cliFailoverClient) DefaultModel() string { return c.name + "-model" }

func TestCLIRunFallsBackAndLabelsSuccessfulTarget(t *testing.T) {
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "openai"}, ProviderConfig: appconfig.ProviderConfig{
		CLIConfigs: []appconfig.SurfaceConfig{{Provider: "openai", Model: "primary", FallbackProvider: "ollama", FallbackModel: "local"}},
	}}
	factory := func(cfg appconfig.App) (llm.Client, error) {
		if cfg.Provider == "openai" {
			return &cliFailoverClient{name: "openai", err: &llm.HTTPError{Provider: "openai", Status: 503}}, nil
		}
		return &cliFailoverClient{name: "ollama", text: "fallback answer"}, nil
	}
	jobs, err := buildCLIJobs(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var out, meta bytes.Buffer
	if err := runCLIJobs(context.Background(), jobs, []llm.Message{{Role: "user", Content: "unique fallback"}}, "unique fallback", &out, &meta, factory, nil); err != nil {
		t.Fatal(err)
	}
	if out.String() != "fallback answer" {
		t.Fatalf("output = %q", out.String())
	}
	if !strings.Contains(meta.String(), "provider=ollama model=local") {
		t.Fatalf("successful target missing from summary: %q", meta.String())
	}
	if _, _, ok := lookupCLIResponseCache(context.Background(), newCLIResponseCacheKey("ollama", "local", jobs[0].req, []llm.Message{{Role: "user", Content: "unique fallback"}})); !ok {
		t.Fatal("successful fallback response was not cached under its target")
	}
}

func TestCLINonEligibleErrorDoesNotFallBack(t *testing.T) {
	calls := 0
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "openai"}, ProviderConfig: appconfig.ProviderConfig{
		CLIConfigs: []appconfig.SurfaceConfig{{Provider: "openai", FallbackProvider: "ollama"}},
	}}
	factory := func(cfg appconfig.App) (llm.Client, error) {
		calls++
		if cfg.Provider == "openai" {
			return &cliFailoverClient{name: "openai", err: &llm.HTTPError{Provider: "openai", Status: 400}}, nil
		}
		return &cliFailoverClient{name: "ollama", text: "unexpected"}, nil
	}
	jobs, _ := buildCLIJobs(cfg)
	err := runCLIJobs(context.Background(), jobs, nil, "invalid", &bytes.Buffer{}, &bytes.Buffer{}, factory, nil)
	if err == nil || calls != 2 {
		t.Fatalf("error=%v factory calls=%d, want primary and target construction only", err, calls)
	}
	if strings.Contains(err.Error(), "unexpected") {
		t.Fatal("fallback response was used for a non-eligible error")
	}
}

func TestCLIPreparedPrimarySurvivesUnavailableFallback(t *testing.T) {
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "openai"}, ProviderConfig: appconfig.ProviderConfig{
		CLIConfigs: []appconfig.SurfaceConfig{{Provider: "openai", FallbackProvider: "ollama"}},
	}}
	jobs, _ := buildCLIJobs(cfg)
	var calls int
	factory := func(cfg appconfig.App) (llm.Client, error) {
		calls++
		if cfg.Provider == "ollama" {
			return nil, errors.New("local Ollama unavailable")
		}
		return &cliFailoverClient{name: "openai", text: "primary answer"}, nil
	}
	var out bytes.Buffer
	if err := runCLIJobs(context.Background(), jobs, nil, "primary survives", &out, &bytes.Buffer{}, factory, nil); err != nil {
		t.Fatal(err)
	}
	if out.String() != "primary answer" || calls != 2 {
		t.Fatalf("output=%q factory calls=%d", out.String(), calls)
	}
}

func TestCLIFallbackCacheHitSkipsTargetConstruction(t *testing.T) {
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "openai"}, ProviderConfig: appconfig.ProviderConfig{
		CLIConfigs: []appconfig.SurfaceConfig{{Provider: "openai", Model: "cloud", FallbackProvider: "ollama", FallbackModel: "local"}},
	}}
	jobs, _ := buildCLIJobs(cfg)
	msgs := []llm.Message{{Role: "user", Content: "cached fallback"}}
	storeCLIResponseCache(context.Background(), newCLIResponseCacheKey("ollama", "local", jobs[0].req, msgs), "cached answer")
	var calls int
	factory := func(appconfig.App) (llm.Client, error) {
		calls++
		return nil, errors.New("must not construct on cache hit")
	}
	var out bytes.Buffer
	if err := runCLIJobs(context.Background(), jobs, msgs, "cached fallback", &out, &bytes.Buffer{}, factory, nil); err != nil {
		t.Fatal(err)
	}
	if out.String() != "cached answer" || calls != 0 {
		t.Fatalf("output=%q factory calls=%d", out.String(), calls)
	}
}

func TestCLIFallbackCacheUsesResponderOptions(t *testing.T) {
	primaryTemperature, fallbackTemperature := 0.2, 0.7
	cfg := appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "cloud"}, ProviderConfig: appconfig.ProviderConfig{
		ProviderProfiles: map[string]appconfig.ProviderProfile{
			"cloud": {Type: "openai", Model: "same", Temperature: &primaryTemperature},
			"local": {Type: "ollama", Model: "same", Temperature: &fallbackTemperature},
		},
		CLIConfigs: []appconfig.SurfaceConfig{{Provider: "cloud", Model: "same", FallbackProvider: "local", FallbackModel: "same"}},
	}}
	jobs, _ := buildCLIJobs(cfg)
	msgs := []llm.Message{{Role: "user", Content: "cache option identity"}}
	factory := func(cfg appconfig.App) (llm.Client, error) {
		if cfg.Provider == "openai" {
			return &cliFailoverClient{name: "openai", err: &llm.HTTPError{Provider: "openai", Status: 503}}, nil
		}
		return &cliFailoverClient{name: "ollama", text: "local answer"}, nil
	}
	var out bytes.Buffer
	if err := runCLIJobs(context.Background(), jobs, msgs, "cache option identity", &out, &bytes.Buffer{}, factory, nil); err != nil {
		t.Fatal(err)
	}
	fallbackReq := buildCLIRequest(appconfig.SurfaceConfig{Provider: "local", Model: "same"}, "local", llmutils.ConfigForProvider(jobs[0].cfg, "local", "same"))
	if _, _, ok := lookupCLIResponseCache(context.Background(), newCLIResponseCacheKey("ollama", "same", fallbackReq, msgs)); !ok {
		t.Fatal("fallback cache entry did not preserve responder options")
	}
}

func TestCLIMultiHeaderPrecedesColumnOutput(t *testing.T) {
	jobs := []cliJob{{index: 0, provider: "openai", cfg: appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "openai"}}, req: requestArgs{model: "cloud"}}, {index: 1, provider: "ollama", cfg: appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "ollama"}}, req: requestArgs{model: "local"}}}
	factory := func(cfg appconfig.App) (llm.Client, error) {
		return &cliFailoverClient{name: cfg.Provider, text: "answer"}, nil
	}
	var combined strings.Builder
	_, _ = executeCLIJobs(context.Background(), jobs, nil, "header order", &combined, &combined, false, factory, nil)
	if header, output := strings.Index(combined.String(), "openai:"), strings.Index(combined.String(), "answer"); header < 0 || output < 0 || header > output {
		t.Fatalf("header must precede output: %q", combined.String())
	}
}

func TestCLISinglePrimaryStreamsBeforeCompletion(t *testing.T) {
	client := &blockingCLIStreamer{started: make(chan struct{}), release: make(chan struct{})}
	job := cliJob{index: 0, provider: "stream", cfg: appconfig.App{CoreConfig: appconfig.CoreConfig{Provider: "stream"}}, req: requestArgs{model: "model"}, entry: appconfig.SurfaceConfig{Provider: "stream", Model: "model"}}
	factory := func(appconfig.App) (llm.Client, error) { return client, nil }
	var out, meta bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = executeCLIJobs(context.Background(), []cliJob{job}, nil, "stream", &out, &meta, true, factory, nil)
		close(done)
	}()
	<-client.started
	if !strings.Contains(out.String(), "chunk") {
		t.Fatal("single primary output was buffered until completion")
	}
	close(client.release)
	<-done
}

func TestCLIStreamingDoesNotMixFailedPrimary(t *testing.T) {
	primary := &cliStreamFailoverClient{cliFailoverClient: cliFailoverClient{name: "openai", err: errors.New("connection refused")}, chunks: []string{"partial"}}
	fallback := &cliFailoverClient{name: "ollama", text: "complete"}
	target, err := runCLIWithFailover(context.Background(), []llm.Target{{Name: "openai", Model: "p", Client: primary}, {Name: "ollama", Model: "f", Client: fallback}}, nil, new(strings.Builder))
	if target.Name != "openai" || err == nil {
		t.Fatalf("expected partial primary failure, target=%+v err=%v", target, err)
	}
	var out strings.Builder
	target, err = runCLIWithFailover(context.Background(), []llm.Target{{Name: "openai", Model: "p", Client: &cliFailoverClient{name: "openai", err: &llm.HTTPError{Provider: "openai", Status: 503}}}, {Name: "ollama", Model: "f", Client: fallback}}, nil, &out)
	if err != nil || target.Name != "ollama" || out.String() != "complete" {
		t.Fatalf("expected clean fallback stream, target=%+v out=%q err=%v", target, out.String(), err)
	}
}

type cliStreamFailoverClient struct {
	cliFailoverClient
	chunks []string
}

type blockingCLIStreamer struct {
	started chan struct{}
	release chan struct{}
}

func (c *blockingCLIStreamer) Chat(context.Context, []llm.Message, ...llm.RequestOption) (string, error) {
	return "chunk", nil
}

func (c *blockingCLIStreamer) ChatStream(_ context.Context, _ []llm.Message, onDelta func(string), _ ...llm.RequestOption) error {
	onDelta("chunk")
	close(c.started)
	<-c.release
	return nil
}

func (c *blockingCLIStreamer) Name() string         { return "stream" }
func (c *blockingCLIStreamer) DefaultModel() string { return "model" }

func (c *cliStreamFailoverClient) ChatStream(_ context.Context, _ []llm.Message, onDelta func(string), _ ...llm.RequestOption) error {
	for _, chunk := range c.chunks {
		onDelta(chunk)
	}
	return c.err
}
