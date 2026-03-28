package provider_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dust/neo-code/internal/config"
	"github.com/dust/neo-code/internal/provider"
	"github.com/dust/neo-code/internal/provider/builtin"
	"github.com/dust/neo-code/internal/provider/openai"
)

type stubProvider struct{}

func (stubProvider) Chat(ctx context.Context, req provider.ChatRequest, events chan<- provider.StreamEvent) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}

func stubDriver(driverType string, models ...string) provider.DriverDefinition {
	return provider.DriverDefinition{
		Name: driverType,
		Build: func(ctx context.Context, cfg config.ResolvedProviderConfig) (provider.Provider, error) {
			return stubProvider{}, nil
		},
		Catalog: func(cfg config.ProviderConfig) (provider.ProviderCatalogItem, error) {
			items := make([]provider.ModelDescriptor, 0, len(models))
			for _, model := range models {
				items = append(items, provider.ModelDescriptor{
					ID:   model,
					Name: model,
				})
			}
			return provider.ProviderCatalogItem{
				ID:        cfg.Name,
				Name:      strings.ToUpper(driverType),
				APIKeyEnv: cfg.APIKeyEnv,
				Models:    items,
			}, nil
		},
	}
}

func newTestRegistry(t *testing.T) *provider.Registry {
	t.Helper()

	registry := provider.NewRegistry()
	if err := registry.Register(openai.DriverDefinition()); err != nil {
		t.Fatalf("register openai driver: %v", err)
	}
	return registry
}

func newTestManager(t *testing.T) *config.Manager {
	t.Helper()

	manager := config.NewManager(config.NewLoader(t.TempDir(), builtin.DefaultConfig()))
	if _, err := manager.Load(context.Background()); err != nil {
		t.Fatalf("load config: %v", err)
	}
	return manager
}

func TestRegistryBuildsRegisteredDriverCaseInsensitively(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry(t)
	got, err := registry.Build(context.Background(), config.ResolvedProviderConfig{
		ProviderConfig: config.ProviderConfig{
			Name:      "openai-main",
			Driver:    "OPENAI",
			BaseURL:   openai.DefaultBaseURL,
			Model:     openai.DefaultModel,
			APIKeyEnv: openai.DefaultAPIKeyEnv,
		},
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if _, ok := got.(*openai.Provider); !ok {
		t.Fatalf("expected openai.Provider, got %T", got)
	}
}

func TestRegistryCatalogNormalizesConfiguredProvider(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry(t)
	item, err := registry.Catalog(config.ProviderConfig{
		Name:      "openai-main",
		Driver:    openai.DriverName,
		BaseURL:   openai.DefaultBaseURL,
		Model:     "custom-model",
		APIKeyEnv: openai.DefaultAPIKeyEnv,
	})
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	if item.ID != "openai-main" {
		t.Fatalf("expected catalog id %q, got %q", "openai-main", item.ID)
	}
	if item.APIKeyEnv != openai.DefaultAPIKeyEnv {
		t.Fatalf("expected env key %q, got %q", openai.DefaultAPIKeyEnv, item.APIKeyEnv)
	}

	foundCustom := 0
	for _, model := range item.Models {
		if model.ID == "custom-model" {
			foundCustom++
		}
	}
	if foundCustom != 1 {
		t.Fatalf("expected configured model to appear once, got %d", foundCustom)
	}
}

func TestRegistryUnknownDriverReturnsTypedError(t *testing.T) {
	t.Parallel()

	registry := provider.NewRegistry()
	_, err := registry.Catalog(config.ProviderConfig{Driver: "missing"})
	if !errors.Is(err, provider.ErrDriverNotFound) {
		t.Fatalf("expected ErrDriverNotFound, got %v", err)
	}
}

func TestServiceListProvidersFiltersUnsupportedDrivers(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	if err := manager.Update(context.Background(), func(cfg *config.Config) error {
		cfg.Providers = append(cfg.Providers, config.ProviderConfig{
			Name:      "unsupported",
			Driver:    "custom",
			BaseURL:   "https://example.com",
			Model:     "custom-model",
			APIKeyEnv: "CUSTOM_API_KEY",
		})
		return nil
	}); err != nil {
		t.Fatalf("append provider: %v", err)
	}

	service := provider.NewService(manager, newTestRegistry(t))
	items, err := service.ListProviders(context.Background())
	if err != nil {
		t.Fatalf("ListProviders() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected only supported providers, got %d", len(items))
	}
	if items[0].ID != openai.DriverName {
		t.Fatalf("expected supported provider %q, got %q", openai.DriverName, items[0].ID)
	}
}

func TestServiceSelectProviderAndSetCurrentModel(t *testing.T) {
	manager := newTestManager(t)
	registry := newTestRegistry(t)
	if err := registry.Register(stubDriver("custom", "custom-model")); err != nil {
		t.Fatalf("register stub driver: %v", err)
	}

	if err := manager.Update(context.Background(), func(cfg *config.Config) error {
		cfg.CurrentModel = "gpt-5.4"
		cfg.Providers = append(cfg.Providers, config.ProviderConfig{
			Name:      "custom-main",
			Driver:    "custom",
			BaseURL:   "https://example.com",
			Model:     "custom-model",
			APIKeyEnv: "CUSTOM_API_KEY",
		})
		return nil
	}); err != nil {
		t.Fatalf("append custom provider: %v", err)
	}

	service := provider.NewService(manager, registry)

	selection, err := service.SelectProvider(context.Background(), "custom-main")
	if err != nil {
		t.Fatalf("SelectProvider() error = %v", err)
	}
	if selection.ProviderID != "custom-main" || selection.ModelID != "custom-model" {
		t.Fatalf("unexpected selection after switch: %+v", selection)
	}

	if _, err := service.SetCurrentModel(context.Background(), "missing-model"); !errors.Is(err, provider.ErrModelNotFound) {
		t.Fatalf("expected ErrModelNotFound, got %v", err)
	}

	selection, err = service.SelectProvider(context.Background(), openai.DriverName)
	if err != nil {
		t.Fatalf("SelectProvider(openai) error = %v", err)
	}
	if selection.ProviderID != openai.DriverName {
		t.Fatalf("expected selected provider %q, got %+v", openai.DriverName, selection)
	}

	selection, err = service.SetCurrentModel(context.Background(), "gpt-4o")
	if err != nil {
		t.Fatalf("SetCurrentModel() error = %v", err)
	}
	if selection.ModelID != "gpt-4o" {
		t.Fatalf("expected selected model %q, got %+v", "gpt-4o", selection)
	}

	cfg := manager.Get()
	selected, err := cfg.SelectedProviderConfig()
	if err != nil {
		t.Fatalf("SelectedProviderConfig() error = %v", err)
	}
	if selected.Model != "gpt-4o" || cfg.CurrentModel != "gpt-4o" {
		t.Fatalf("expected config model to be updated, got provider=%q current=%q", selected.Model, cfg.CurrentModel)
	}
}
