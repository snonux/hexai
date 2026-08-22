package hexailsp

import (
	"context"
	"log"

	"github.com/snonux/hexai/internal/appconfig"
	"github.com/snonux/hexai/internal/ignore"
	"github.com/snonux/hexai/internal/llm"
	"github.com/snonux/hexai/internal/llmutils"
	"github.com/snonux/hexai/internal/logging"
	"github.com/snonux/hexai/internal/lsp"
	"github.com/snonux/hexai/internal/runtimeconfig"
)

type configLoader func(context.Context, *log.Logger, appconfig.LoadOptions) appconfig.App

type clientBuilder func(appconfig.App, llm.Client) llm.Client

type configStoreFactory func(appconfig.App) *runtimeconfig.Store

type ignoreCheckerFactory func(appconfig.App) *ignore.Checker

type runDependencies struct {
	loadConfig       configLoader
	buildClient      clientBuilder
	newConfigStore   configStoreFactory
	newIgnoreChecker ignoreCheckerFactory
	statusSink       lsp.StatusSink
}

func defaultRunDependencies() runDependencies {
	return runDependencies{
		loadConfig:       appconfig.LoadWithOptions,
		buildClient:      defaultClientBuilder,
		newConfigStore:   runtimeconfig.New,
		newIgnoreChecker: defaultIgnoreCheckerFactory,
		statusSink:       tmuxStatusSink{},
	}
}

func normalizeRunDependencies(deps runDependencies) runDependencies {
	if deps.loadConfig == nil {
		deps.loadConfig = appconfig.LoadWithOptions
	}
	if deps.buildClient == nil {
		deps.buildClient = defaultClientBuilder
	}
	if deps.newConfigStore == nil {
		deps.newConfigStore = runtimeconfig.New
	}
	if deps.newIgnoreChecker == nil {
		deps.newIgnoreChecker = defaultIgnoreCheckerFactory
	}
	if deps.statusSink == nil {
		deps.statusSink = tmuxStatusSink{}
	}
	return deps
}

func defaultClientBuilder(cfg appconfig.App, client llm.Client) llm.Client {
	if client != nil {
		return client
	}
	c, err := llmutils.NewClientFromApp(cfg)
	if err != nil {
		logging.Logf("lsp ", "llm disabled: %v", err)
		return nil
	}
	logging.Logf("lsp ", "llm enabled provider=%s model=%s", c.Name(), c.DefaultModel())
	return c
}

func defaultIgnoreCheckerFactory(cfg appconfig.App) *ignore.Checker {
	gitRoot := appconfig.FindGitRoot()
	useGI := cfg.IgnoreGitignore == nil || *cfg.IgnoreGitignore
	return ignore.New(gitRoot, useGI, cfg.IgnoreExtraPatterns)
}
