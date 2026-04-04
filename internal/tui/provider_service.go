package tui

import (
	"context"

	"neo-code/internal/config"
)

// ProviderController abstracts provider and model selection operations.
// Implemented by config.SelectionOrchestrator.
type ProviderController interface {
	ListProviders(ctx context.Context) ([]config.ProviderCatalogItem, error)
	SelectProvider(ctx context.Context, providerID string) (config.ProviderSelection, error)
	ListModels(ctx context.Context) ([]config.ModelDescriptor, error)
	ListModelsSnapshot(ctx context.Context) ([]config.ModelDescriptor, error)
	SetCurrentModel(ctx context.Context, modelID string) (config.ProviderSelection, error)
}
