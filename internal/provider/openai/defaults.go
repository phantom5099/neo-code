package openai

import "github.com/dust/neo-code/internal/config"

const (
	DriverName       = "openai"
	DefaultBaseURL   = "https://api.openai.com/v1"
	DefaultModel     = "gpt-4.1"
	DefaultAPIKeyEnv = "OPENAI_API_KEY"
)

func DefaultConfig() config.ProviderConfig {
	return config.ProviderConfig{
		Name:      DriverName,
		Driver:    DriverName,
		BaseURL:   DefaultBaseURL,
		Model:     DefaultModel,
		APIKeyEnv: DefaultAPIKeyEnv,
	}
}
