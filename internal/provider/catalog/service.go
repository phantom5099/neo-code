package catalog

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"neo-code/internal/provider"
	providertypes "neo-code/internal/provider/types"
)

const (
	defaultTTL               = 24 * time.Hour
	defaultBackgroundTimeout = 30 * time.Second
)

type Service struct {
	registry          *provider.Registry
	store             Store
	catalogTTL        time.Duration
	backgroundTimeout time.Duration
	now               func() time.Time

	refreshMu    sync.Mutex
	inFlightByID map[string]struct{}
}

func NewService(baseDir string, registry *provider.Registry, store Store) *Service {
	if store == nil && strings.TrimSpace(baseDir) != "" {
		store = newJSONStore(baseDir)
	}

	return &Service{
		registry:          registry,
		store:             store,
		catalogTTL:        defaultTTL,
		backgroundTimeout: defaultBackgroundTimeout,
		now:               time.Now,
		inFlightByID:      map[string]struct{}{},
	}
}

func (s *Service) ListProviderModels(ctx context.Context, input provider.CatalogInput) ([]providertypes.ModelDescriptor, error) {
	return s.listProviderModels(ctx, input, queryOptions{
		allowSyncRefresh: true,
		queueRefresh:     true,
	})
}

func (s *Service) ListProviderModelsSnapshot(ctx context.Context, input provider.CatalogInput) ([]providertypes.ModelDescriptor, error) {
	return s.listProviderModels(ctx, input, queryOptions{
		queueRefresh: true,
	})
}

func (s *Service) ListProviderModelsCached(ctx context.Context, input provider.CatalogInput) ([]providertypes.ModelDescriptor, error) {
	return s.listProviderModels(ctx, input, queryOptions{})
}

func (s *Service) listProviderModels(
	ctx context.Context,
	input provider.CatalogInput,
	options queryOptions,
) ([]providertypes.ModelDescriptor, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return s.modelsForProvider(ctx, input, options)
}

func (s *Service) validate() error {
	if s == nil {
		return errors.New("provider catalog: service is nil")
	}
	if s.registry == nil {
		return errors.New("provider catalog: registry is nil")
	}
	return nil
}

type queryOptions struct {
	allowSyncRefresh bool
	queueRefresh     bool
}

type catalogSnapshot struct {
	models   []providertypes.ModelDescriptor
	ok       bool
	expired  bool
	identity provider.ProviderIdentity
}

func (s *Service) modelsForProvider(ctx context.Context, input provider.CatalogInput, options queryOptions) ([]providertypes.ModelDescriptor, error) {
	configuredModels := providertypes.MergeModelDescriptors(input.ConfiguredModels)
	defaultModels := providertypes.MergeModelDescriptors(input.DefaultModels)
	snapshot := s.catalogSnapshot(ctx, input)

	models := snapshot.models
	catalogOK := snapshot.ok
	performedSyncRefresh := false
	if !catalogOK && options.allowSyncRefresh {
		discovered, err := s.discoverAndPersist(ctx, input)
		if err != nil {
			if len(defaultModels) == 0 {
				return nil, err
			}
		} else {
			models = discovered
			catalogOK = true
			performedSyncRefresh = true
		}
	}

	if options.queueRefresh && snapshot.expired {
		s.queueRefresh(input)
	}
	if options.queueRefresh && !snapshot.ok && !performedSyncRefresh {
		s.queueRefresh(input)
	}

	if !catalogOK {
		if len(defaultModels) == 0 {
			return nil, nil
		}
		return providertypes.MergeModelDescriptors(configuredModels, defaultModels), nil
	}
	return providertypes.MergeModelDescriptors(configuredModels, models, defaultModels), nil
}

func (s *Service) catalogSnapshot(ctx context.Context, input provider.CatalogInput) catalogSnapshot {
	identity := input.Identity
	modelCatalog, err := s.loadCatalog(ctx, identity)
	if err != nil {
		return catalogSnapshot{identity: identity}
	}
	return catalogSnapshot{
		models:   modelCatalog.Models,
		ok:       true,
		expired:  modelCatalog.Expired(s.now()),
		identity: identity,
	}
}

func (s *Service) loadCatalog(ctx context.Context, identity provider.ProviderIdentity) (ModelCatalog, error) {
	if s.store == nil {
		return ModelCatalog{}, ErrCatalogNotFound
	}
	return s.store.Load(ctx, identity)
}

func (s *Service) discoverAndPersist(ctx context.Context, input provider.CatalogInput) ([]providertypes.ModelDescriptor, error) {
	driverType := strings.TrimSpace(input.Identity.Driver)
	if !s.registry.Supports(driverType) {
		return nil, nil
	}

	if input.ResolveDiscoveryConfig == nil {
		return nil, errors.New("provider catalog: discovery config resolver is nil")
	}

	runtimeCfg, err := input.ResolveDiscoveryConfig()
	if err != nil {
		return nil, err
	}

	discovered, err := s.registry.DiscoverModels(ctx, runtimeCfg)
	if err != nil {
		return nil, err
	}

	discovered = providertypes.MergeModelDescriptors(discovered)
	if s.store == nil {
		return discovered, nil
	}

	now := s.now()
	_ = s.store.Save(ctx, ModelCatalog{
		SchemaVersion: schemaVersion,
		Identity:      input.Identity,
		FetchedAt:     now,
		ExpiresAt:     now.Add(s.catalogTTL),
		Models:        discovered,
	})
	return discovered, nil
}

func (s *Service) queueRefresh(input provider.CatalogInput) {
	driverType := strings.TrimSpace(input.Identity.Driver)
	if s.store == nil || !s.registry.Supports(driverType) {
		return
	}
	identity := input.Identity
	if identity.Driver == "" || identity.BaseURL == "" {
		return
	}

	key := identity.Key()
	s.refreshMu.Lock()
	if _, exists := s.inFlightByID[key]; exists {
		s.refreshMu.Unlock()
		return
	}
	s.inFlightByID[key] = struct{}{}
	s.refreshMu.Unlock()

	go func() {
		defer func() {
			s.refreshMu.Lock()
			delete(s.inFlightByID, key)
			s.refreshMu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), s.backgroundTimeout)
		defer cancel()
		_, _ = s.discoverAndPersist(ctx, input)
	}()
}
