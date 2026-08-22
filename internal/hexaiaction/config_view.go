package hexaiaction

import "github.com/snonux/hexai/internal/appconfig"

// actionConfig narrows dependencies to the config sections needed by actions.
type actionConfig interface {
	CoreSection() appconfig.CoreConfig
	ProviderSection() appconfig.ProviderConfig
	PromptSection() appconfig.PromptConfig
}

func actionConfigAsApp(cfg actionConfig) appconfig.App {
	app := appconfig.App{}
	app.ApplyCoreSection(cfg.CoreSection())
	app.ApplyProviderSection(cfg.ProviderSection())
	app.ApplyPromptSection(cfg.PromptSection())
	return app
}
