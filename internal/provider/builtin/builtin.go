package builtin

import (
	"errors"

	"github.com/dust/neo-code/internal/config"
	"github.com/dust/neo-code/internal/provider"
	"github.com/dust/neo-code/internal/provider/openai"
)

func DefaultConfig() *config.Config {
	cfg := config.Default()
	defaultProvider := openai.DefaultConfig()
	cfg.Providers = []config.ProviderConfig{defaultProvider}
	cfg.SelectedProvider = defaultProvider.Name
	cfg.CurrentModel = defaultProvider.Model
	return cfg
}

func NewRegistry() (*provider.Registry, error) {
	registry := provider.NewRegistry()
	if err := Register(registry); err != nil {
		return nil, err
	}
	return registry, nil
}

func Register(registry *provider.Registry) error {
	if registry == nil {
		return errors.New("builtin provider registry is nil")
	}
	return registry.Register(openai.DriverDefinition())
}
