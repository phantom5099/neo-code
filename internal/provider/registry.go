package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dust/neo-code/internal/config"
)

type Builder func(ctx context.Context, cfg config.ResolvedProviderConfig) (Provider, error)

type CatalogBuilder func(cfg config.ProviderConfig) (ProviderCatalogItem, error)

type DriverDefinition struct {
	Name    string
	Build   Builder
	Catalog CatalogBuilder
}

type Registry struct {
	drivers map[string]DriverDefinition
}

func NewRegistry() *Registry {
	return &Registry{drivers: map[string]DriverDefinition{}}
}

func (r *Registry) Register(driver DriverDefinition) error {
	driverType := normalizeKey(driver.Name)
	if driverType == "" {
		return errors.New("provider: driver name is empty")
	}
	if driver.Build == nil {
		return fmt.Errorf("provider: driver %q build func is nil", driver.Name)
	}
	if driver.Catalog == nil {
		return fmt.Errorf("provider: driver %q catalog func is nil", driver.Name)
	}
	r.drivers[driverType] = driver
	return nil
}

func (r *Registry) Build(ctx context.Context, cfg config.ResolvedProviderConfig) (Provider, error) {
	driver, err := r.driver(cfg.Driver)
	if err != nil {
		return nil, err
	}
	return driver.Build(ctx, cfg)
}

func (r *Registry) Catalog(cfg config.ProviderConfig) (ProviderCatalogItem, error) {
	driver, err := r.driver(cfg.Driver)
	if err != nil {
		return ProviderCatalogItem{}, err
	}

	item, err := driver.Catalog(cfg)
	if err != nil {
		return ProviderCatalogItem{}, err
	}
	return normalizeCatalogItem(item, cfg.Name, cfg.APIKeyEnv), nil
}

func (r *Registry) Supports(driverType string) bool {
	_, err := r.driver(driverType)
	return err == nil
}

func (r *Registry) driver(driverType string) (DriverDefinition, error) {
	if r == nil {
		return DriverDefinition{}, ErrDriverNotFound
	}
	driver, ok := r.drivers[normalizeKey(driverType)]
	if !ok {
		return DriverDefinition{}, fmt.Errorf("%w: %s", ErrDriverNotFound, strings.TrimSpace(driverType))
	}
	return driver, nil
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
