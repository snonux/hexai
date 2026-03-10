package appconfig

const configPathFallback = "$XDG_CONFIG_HOME/hexai/config.toml"

// DefaultConfigPath returns the resolved config path or the documented XDG
// fallback when the real path cannot be determined.
func DefaultConfigPath() string {
	return configPathOrFallback(ConfigPath)
}

func configPathOrFallback(resolve func() (string, error)) string {
	path, err := resolve()
	if err != nil {
		return configPathFallback
	}
	return path
}
