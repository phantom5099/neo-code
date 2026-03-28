package openai

import (
	"context"
	"strings"

	"github.com/dust/neo-code/internal/config"
	"github.com/dust/neo-code/internal/provider"
)

func DriverDefinition() provider.DriverDefinition {
	return provider.DriverDefinition{
		Name: DriverName,
		Build: func(ctx context.Context, cfg config.ResolvedProviderConfig) (provider.Provider, error) {
			return New(cfg)
		},
		Catalog: CatalogItem,
	}
}

func CatalogItem(cfg config.ProviderConfig) (provider.ProviderCatalogItem, error) {
	models := []provider.ModelDescriptor{
		{
			ID:          DefaultModel,
			Name:        DefaultModel,
			Description: "Stable OpenAI-compatible default model",
		},
		{
			ID:          "gpt-4o",
			Name:        "gpt-4o",
			Description: "Fast general-purpose OpenAI-compatible model",
		},
		{
			ID:          "gpt-5.3-codex",
			Name:        "gpt-5.3-codex",
			Description: "Code-focused OpenAI-compatible model",
		},
		{
			ID:          "gpt-5.4",
			Name:        "gpt-5.4",
			Description: "Frontier reasoning and coding model",
		},
	}

	if configured := strings.TrimSpace(cfg.Model); configured != "" {
		models = append(models, provider.ModelDescriptor{
			ID:          configured,
			Name:        configured,
			Description: "Configured default model",
		})
	}

	return provider.ProviderCatalogItem{
		ID:          cfg.Name,
		Name:        cfg.Name,
		Description: "OpenAI-compatible chat completions provider.",
		APIKeyEnv:   cfg.APIKeyEnv,
		Models:      models,
	}, nil
}
