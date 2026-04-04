package config

import (
	"context"
	"errors"
	"strings"
)

// DriverSupporter checks whether a driver type is available.
// Implemented by provider.Registry; kept as an interface to avoid coupling config→provider.
type DriverSupporter interface {
	Supports(driver string) bool
}

// ModelCatalog fetches model lists for a given provider configuration.
// Implemented by provider/catalog.Service; kept as an interface to avoid coupling config→provider.
type ModelCatalog interface {
	ListProviderModels(ctx context.Context, providerCfg ProviderConfig) ([]ModelDescriptor, error)
	ListProviderModelsSnapshot(ctx context.Context, providerCfg ProviderConfig) ([]ModelDescriptor, error)
	ListProviderModelsCached(ctx context.Context, providerCfg ProviderConfig) ([]ModelDescriptor, error)
}

// SelectionService manages provider and model selection state in configuration.
// It is responsible for writing selection changes (selected_provider, current_model)
// to the config store, while validation and data fetching remain in the caller.
type SelectionService struct {
	manager *Manager
}

// NewSelectionService creates a SelectionService backed by the given config Manager.
func NewSelectionService(manager *Manager) *SelectionService {
	if manager == nil {
		panic("config: manager is nil")
	}
	return &SelectionService{manager: manager}
}

// SelectProvider switches the active provider and resolves the current model.
// It validates that targetProviderName exists and driverSupport confirms driver compatibility,
// then updates selected_provider and resolves current_model against available models.
//
// Parameters:
//   - targetProviderName: the provider name to switch to
//   - models: available models for the target provider (from catalog)
//   - defaultModel: the target provider's default model (fallback)
//   - driverSupport: function to check if a driver is supported by the registry
func (svc *SelectionService) SelectProvider(
	ctx context.Context,
	targetProviderName string,
	models []ModelDescriptor,
	defaultModel string,
	driverSupport func(string) bool,
) (ProviderSelection, error) {
	if targetProviderName = strings.TrimSpace(targetProviderName); targetProviderName == "" {
		return ProviderSelection{}, errors.New("config: provider name is empty")
	}

	var selection ProviderSelection
	err := svc.manager.Update(ctx, func(cfg *Config) error {
		selected, err := cfg.ProviderByName(targetProviderName)
		if err != nil {
			return err
		}
		if driverSupport != nil && !driverSupport(selected.Driver) {
			return ErrDriverNotSupported
		}

		cfg.SelectedProvider = selected.Name
		cfg.CurrentModel, _ = ResolveCurrentModel(cfg.CurrentModel, models, selected.Model)
		selection = ProviderSelection{
			ProviderID: cfg.SelectedProvider,
			ModelID:    cfg.CurrentModel,
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrDriverNotSupported) {
			return ProviderSelection{}, ErrDriverNotSupported
		}
		return ProviderSelection{}, err
	}
	return selection, nil
}

// SetCurrentModel switches the active model for the currently selected provider.
// It validates that modelID is non-empty and exists in the provided models list.
func (svc *SelectionService) SetCurrentModel(
	ctx context.Context,
	modelID string,
	models []ModelDescriptor,
) (ProviderSelection, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ProviderSelection{}, ErrModelNotFound
	}
	if !ContainsModelDescriptorID(models, modelID) {
		return ProviderSelection{}, ErrModelNotFound
	}

	var selection ProviderSelection
	err := svc.manager.Update(ctx, func(cfg *Config) error {
		if _, err := cfg.SelectedProviderConfig(); err != nil {
			return err
		}
		cfg.CurrentModel = modelID
		selection = ProviderSelection{
			ProviderID: cfg.SelectedProvider,
			ModelID:    cfg.CurrentModel,
		}
		return nil
	})
	if err != nil {
		return ProviderSelection{}, err
	}
	return selection, nil
}

// EnsureSelection validates the current model against available models,
// repairing it to the fallback if the current model is no longer valid.
// Returns whether the model was changed and any error.
func (svc *SelectionService) EnsureSelection(
	ctx context.Context,
	models []ModelDescriptor,
	fallback string,
) (ProviderSelection, bool, error) {
	snapshot := svc.manager.Get()
	nextModel, changed := ResolveCurrentModel(snapshot.CurrentModel, models, fallback)
	if !changed {
		return ProviderSelection{
			ProviderID: snapshot.SelectedProvider,
			ModelID:    snapshot.CurrentModel,
		}, false, nil
	}

	var selection ProviderSelection
	err := svc.manager.Update(ctx, func(cfg *Config) error {
		if _, err := cfg.SelectedProviderConfig(); err != nil {
			return err
		}
		cfg.CurrentModel = nextModel
		selection = ProviderSelection{
			ProviderID: cfg.SelectedProvider,
			ModelID:    cfg.CurrentModel,
		}
		return nil
	})
	if err != nil {
		return ProviderSelection{}, false, err
	}
	return selection, true, nil
}

// ResolveCurrentModel returns the resolved current model and whether it was changed.
// If currentModel is valid (exists in models), it is returned unchanged.
// Otherwise, fallback is returned if it exists in models.
// If neither is valid, currentModel is returned unchanged.
func ResolveCurrentModel(currentModel string, models []ModelDescriptor, fallback string) (string, bool) {
	currentModel = strings.TrimSpace(currentModel)
	if ContainsModelDescriptorID(models, currentModel) {
		return currentModel, false
	}

	fallback = strings.TrimSpace(fallback)
	if fallback != "" && ContainsModelDescriptorID(models, fallback) {
		return fallback, currentModel != fallback
	}

	return currentModel, false
}

// ContainsModelDescriptorID checks if a model ID exists in the descriptor list (case-insensitive).
func ContainsModelDescriptorID(models []ModelDescriptor, modelID string) bool {
	target := NormalizeKey(modelID)
	if target == "" {
		return false
	}
	for _, model := range models {
		if NormalizeKey(model.ID) == target {
			return true
		}
	}
	return false
}

// --- Orchestrator: full ProviderController implementation ---

// SelectionOrchestrator implements the full provider/model selection workflow
// by composing config.Manager + DriverSupporter + ModelCatalog.
// It is the canonical implementation of tui.ProviderController.
type SelectionOrchestrator struct {
	manager  *Manager
	support  DriverSupporter
	catalogs ModelCatalog
}

// NewSelectionOrchestrator creates an orchestrator backed by config manager,
// driver registry (for support checks), and model catalog (for listing).
func NewSelectionOrchestrator(manager *Manager, support DriverSupporter, catalogs ModelCatalog) *SelectionOrchestrator {
	if manager == nil {
		panic("config: manager is nil")
	}
	return &SelectionOrchestrator{
		manager:  manager,
		support:  support,
		catalogs: catalogs,
	}
}

func (s *SelectionOrchestrator) validate() error {
	if s == nil {
		return errors.New("selection: service is nil")
	}
	if s.manager == nil {
		return errors.New("selection: config manager is nil")
	}
	if s.support == nil {
		return errors.New("selection: driver supporter is nil")
	}
	if s.catalogs == nil {
		return errors.New("selection: catalog service is nil")
	}
	return nil
}

func (s *SelectionOrchestrator) selectedProviderConfig() (ProviderConfig, error) {
	cfg := s.manager.Get()
	return cfg.SelectedProviderConfig()
}

func (s *SelectionOrchestrator) ListProviders(ctx context.Context) ([]ProviderCatalogItem, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cfg := s.manager.Get()
	items := make([]ProviderCatalogItem, 0, len(cfg.Providers))
	for _, providerCfg := range cfg.Providers {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !s.support.Supports(providerCfg.Driver) {
			continue
		}

		models, err := s.catalogs.ListProviderModelsCached(ctx, providerCfg)
		if err != nil {
			return nil, err
		}
		items = append(items, providerCatalogItem(providerCfg, models))
	}

	return items, nil
}

func (s *SelectionOrchestrator) SelectProvider(ctx context.Context, providerName string) (ProviderSelection, error) {
	if err := s.validate(); err != nil {
		return ProviderSelection{}, err
	}

	cfgSnapshot := s.manager.Get()
	providerCfg, err := cfgSnapshot.ProviderByName(providerName)
	if err != nil {
		return ProviderSelection{}, ErrProviderNotFound
	}
	if !s.support.Supports(providerCfg.Driver) {
		return ProviderSelection{}, ErrDriverNotSupported
	}

	models, err := s.catalogs.ListProviderModelsCached(ctx, providerCfg)
	if err != nil {
		return ProviderSelection{}, err
	}

	var selection ProviderSelection
	err = s.manager.Update(ctx, func(cfg *Config) error {
		selected, err := cfg.ProviderByName(providerName)
		if err != nil {
			return ErrProviderNotFound
		}
		if !s.support.Supports(selected.Driver) {
			return ErrDriverNotSupported
		}

		cfg.SelectedProvider = selected.Name
		cfg.CurrentModel, _ = ResolveCurrentModel(cfg.CurrentModel, models, selected.Model)
		selection = selectionFromConfig(*cfg)
		return nil
	})
	if err != nil {
		return ProviderSelection{}, err
	}

	return selection, nil
}

func (s *SelectionOrchestrator) ListModels(ctx context.Context) ([]ModelDescriptor, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	selected, err := s.selectedProviderConfig()
	if err != nil {
		return nil, err
	}
	return s.catalogs.ListProviderModels(ctx, selected)
}

func (s *SelectionOrchestrator) ListModelsSnapshot(ctx context.Context) ([]ModelDescriptor, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	selected, err := s.selectedProviderConfig()
	if err != nil {
		return nil, err
	}
	return s.catalogs.ListProviderModelsSnapshot(ctx, selected)
}

func (s *SelectionOrchestrator) SetCurrentModel(ctx context.Context, modelID string) (ProviderSelection, error) {
	if err := s.validate(); err != nil {
		return ProviderSelection{}, err
	}

	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ProviderSelection{}, ErrModelNotFound
	}

	selected, err := s.selectedProviderConfig()
	if err != nil {
		return ProviderSelection{}, err
	}

	models, err := s.catalogs.ListProviderModelsSnapshot(ctx, selected)
	if err != nil {
		return ProviderSelection{}, err
	}
	if !ContainsModelDescriptorID(models, modelID) {
		return ProviderSelection{}, ErrModelNotFound
	}

	var selection ProviderSelection
	err = s.manager.Update(ctx, func(cfg *Config) error {
		if _, err := cfg.SelectedProviderConfig(); err != nil {
			return err
		}
		cfg.CurrentModel = modelID
		selection = selectionFromConfig(*cfg)
		return nil
	})
	if err != nil {
		return ProviderSelection{}, err
	}

	return selection, nil
}

func (s *SelectionOrchestrator) EnsureSelection(ctx context.Context) (ProviderSelection, error) {
	if err := s.validate(); err != nil {
		return ProviderSelection{}, err
	}
	if err := ctx.Err(); err != nil {
		return ProviderSelection{}, err
	}

	cfgSnapshot := s.manager.Get()
	selected, err := cfgSnapshot.SelectedProviderConfig()
	if err != nil {
		return ProviderSelection{}, err
	}

	models, err := s.catalogs.ListProviderModelsSnapshot(ctx, selected)
	if err != nil {
		return ProviderSelection{}, err
	}
	nextModel, changed := ResolveCurrentModel(cfgSnapshot.CurrentModel, models, selected.Model)
	if !changed {
		return selectionFromConfig(cfgSnapshot), nil
	}

	var selection ProviderSelection
	err = s.manager.Update(ctx, func(cfg *Config) error {
		if _, err := cfg.SelectedProviderConfig(); err != nil {
			return err
		}
		cfg.CurrentModel = nextModel
		selection = selectionFromConfig(*cfg)
		return nil
	})
	if err != nil {
		return ProviderSelection{}, err
	}

	return selection, nil
}

func selectionFromConfig(cfg Config) ProviderSelection {
	return ProviderSelection{
		ProviderID: cfg.SelectedProvider,
		ModelID:    cfg.CurrentModel,
	}
}

func providerCatalogItem(cfg ProviderConfig, models []ModelDescriptor) ProviderCatalogItem {
	return ProviderCatalogItem{
		ID:     strings.TrimSpace(cfg.Name),
		Name:   strings.TrimSpace(cfg.Name),
		Models: MergeModelDescriptors(models),
	}
}
