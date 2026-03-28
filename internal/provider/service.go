package provider

import (
	"context"
	"errors"
	"strings"

	"github.com/dust/neo-code/internal/config"
)

type Service struct {
	manager  *config.Manager
	registry *Registry
}

func NewService(manager *config.Manager, registry *Registry) *Service {
	return &Service{
		manager:  manager,
		registry: registry,
	}
}

func (s *Service) ListProviders(ctx context.Context) ([]ProviderCatalogItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cfg := s.manager.Get()
	items := make([]ProviderCatalogItem, 0, len(cfg.Providers))
	for _, providerCfg := range cfg.Providers {
		item, err := s.registry.Catalog(providerCfg)
		if errors.Is(err, ErrDriverNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

func (s *Service) SelectProvider(ctx context.Context, providerID string) (ProviderSelection, error) {
	var selection ProviderSelection

	err := s.manager.Update(ctx, func(cfg *config.Config) error {
		item, providerCfg, err := s.catalogForProvider(*cfg, providerID)
		if err != nil {
			return err
		}

		cfg.SelectedProvider = providerCfg.Name
		if !containsModel(item.Models, cfg.CurrentModel) {
			cfg.CurrentModel = providerCfg.Model
		}

		for i := range cfg.Providers {
			if strings.EqualFold(strings.TrimSpace(cfg.Providers[i].Name), strings.TrimSpace(providerCfg.Name)) {
				cfg.Providers[i].Model = cfg.CurrentModel
				break
			}
		}

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

func (s *Service) ListModels(ctx context.Context) ([]ModelDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cfg := s.manager.Get()
	selected, err := cfg.SelectedProviderConfig()
	if err != nil {
		return nil, err
	}

	item, err := s.registry.Catalog(selected)
	if errors.Is(err, ErrDriverNotFound) {
		return nil, ErrProviderNotFound
	}
	if err != nil {
		return nil, err
	}

	return append([]ModelDescriptor(nil), item.Models...), nil
}

func (s *Service) SetCurrentModel(ctx context.Context, modelID string) (ProviderSelection, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ProviderSelection{}, ErrModelNotFound
	}

	var selection ProviderSelection
	err := s.manager.Update(ctx, func(cfg *config.Config) error {
		selected, err := cfg.SelectedProviderConfig()
		if err != nil {
			return err
		}

		item, err := s.registry.Catalog(selected)
		if errors.Is(err, ErrDriverNotFound) {
			return ErrProviderNotFound
		}
		if err != nil {
			return err
		}
		if !containsModel(item.Models, modelID) {
			return ErrModelNotFound
		}

		cfg.CurrentModel = modelID
		for i := range cfg.Providers {
			if strings.EqualFold(strings.TrimSpace(cfg.Providers[i].Name), strings.TrimSpace(selected.Name)) {
				cfg.Providers[i].Model = modelID
				break
			}
		}

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

func (s *Service) catalogForProvider(cfg config.Config, providerID string) (ProviderCatalogItem, config.ProviderConfig, error) {
	providerCfg, err := cfg.ProviderByName(providerID)
	if err != nil {
		return ProviderCatalogItem{}, config.ProviderConfig{}, ErrProviderNotFound
	}

	item, err := s.registry.Catalog(providerCfg)
	if errors.Is(err, ErrDriverNotFound) {
		return ProviderCatalogItem{}, config.ProviderConfig{}, ErrProviderNotFound
	}
	if err != nil {
		return ProviderCatalogItem{}, config.ProviderConfig{}, err
	}

	return item, providerCfg, nil
}

func containsModel(models []ModelDescriptor, modelID string) bool {
	target := strings.ToLower(strings.TrimSpace(modelID))
	for _, model := range models {
		if strings.ToLower(strings.TrimSpace(model.ID)) == target {
			return true
		}
	}
	return false
}
