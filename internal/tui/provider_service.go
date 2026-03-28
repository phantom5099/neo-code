package tui

import (
	"context"

	"github.com/dust/neo-code/internal/provider"
)

type ProviderController interface {
	ListProviders(ctx context.Context) ([]provider.ProviderCatalogItem, error)
	SelectProvider(ctx context.Context, providerID string) (provider.ProviderSelection, error)
	ListModels(ctx context.Context) ([]provider.ModelDescriptor, error)
	SetCurrentModel(ctx context.Context, modelID string) (provider.ProviderSelection, error)
}
