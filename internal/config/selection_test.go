package config

import (
	"context"
	"errors"
	"testing"
)

// --- Mocks to avoid importing provider/catalog (which imports config) ---

type mockDriverSupporter struct {
	supported string
}

func (m *mockDriverSupporter) Supports(driver string) bool {
	if m.supported == "" {
		return true // support all
	}
	return driver == m.supported
}

type mockModelCatalog struct {
	models []ModelDescriptor
	err    error
}

func (m *mockModelCatalog) ListProviderModels(_ context.Context, _ ProviderConfig) ([]ModelDescriptor, error) {
	return m.models, m.err
}

func (m *mockModelCatalog) ListProviderModelsSnapshot(_ context.Context, _ ProviderConfig) ([]ModelDescriptor, error) {
	return m.models, m.err
}

func (m *mockModelCatalog) ListProviderModelsCached(_ context.Context, _ ProviderConfig) ([]ModelDescriptor, error) {
	return m.models, m.err
}

// --- Tests ---

func TestSelectionOrchestrator_ValidateErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		service *SelectionOrchestrator
		errMsg  string
	}{
		{
			name:    "nil service",
			service: nil,
			errMsg:  "selection: service is nil",
		},
		{
			name: "nil supporter",
			service: &SelectionOrchestrator{
				manager:  NewManager(NewLoader(t.TempDir(), DefaultConfig())),
				support:  nil,
				catalogs: &mockModelCatalog{},
			},
			errMsg: "selection: driver supporter is nil",
		},
		{
			name: "nil catalogs",
			service: &SelectionOrchestrator{
				manager:  NewManager(NewLoader(t.TempDir(), DefaultConfig())),
				support:  &mockDriverSupporter{},
				catalogs: nil,
			},
			errMsg: "selection: catalog service is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.service.ListProviders(context.Background())
			if err == nil || err.Error() != tt.errMsg {
				t.Fatalf("expected error %q, got %v", tt.errMsg, err)
			}
		})
	}
}

func TestSelectionOrchestrator_OperationsWithCanceledContext(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t, DefaultConfig())
	service := NewSelectionOrchestrator(manager, &mockDriverSupporter{}, &mockModelCatalog{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name string
		fn   func(context.Context) error
	}{
		{
			name: "ListProviders",
			fn: func(ctx context.Context) error {
				_, err := service.ListProviders(ctx)
				return err
			},
		},
		{
			name: "ListModels",
			fn: func(ctx context.Context) error {
				_, err := service.ListModels(ctx)
				return err
			},
		},
		{
			name: "ListModelsSnapshot",
			fn: func(ctx context.Context) error {
				_, err := service.ListModelsSnapshot(ctx)
				return err
			},
		},
		{
			name: "EnsureSelection",
			fn: func(ctx context.Context) error {
				_, err := service.EnsureSelection(ctx)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.fn(ctx)
			if err == nil {
				t.Fatalf("expected error for canceled context")
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("expected context.Canceled, got %v", err)
			}
		})
	}
}

func TestSelectionOrchestrator_SetCurrentModelEmptyModelID(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t, DefaultConfig())
	support := &mockDriverSupporter{supported: OpenAIName}
	catalogs := &mockModelCatalog{models: []ModelDescriptor{{ID: "gpt-4o", Name: "GPT-4o"}}}
	service := NewSelectionOrchestrator(manager, support, catalogs)

	_, err := service.SetCurrentModel(context.Background(), "")
	if !errors.Is(err, ErrModelNotFound) {
		t.Fatalf("expected ErrModelNotFound for empty model ID, got %v", err)
	}

	_, err = service.SetCurrentModel(context.Background(), "   ")
	if !errors.Is(err, ErrModelNotFound) {
		t.Fatalf("expected ErrModelNotFound for whitespace model ID, got %v", err)
	}
}

func TestSelectionOrchestrator_SetCurrentModelRejectsMissing(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t, DefaultConfig())
	support := &mockDriverSupporter{supported: OpenAIName}
	catalogs := &mockModelCatalog{models: []ModelDescriptor{{ID: "gpt-4o"}}}
	service := NewSelectionOrchestrator(manager, support, catalogs)

	_, err := service.SetCurrentModel(context.Background(), "missing-model")
	if !errors.Is(err, ErrModelNotFound) {
		t.Fatalf("expected ErrModelNotFound, got %v", err)
	}
}

func TestSelectionOrchestrator_SelectProviderRejectsUnsupportedDriver(t *testing.T) {
	t.Parallel()

	defaults := DefaultConfig()
	defaults.Providers = []ProviderConfig{{
		Name:      OpenAIName,
		Driver:    "missing-driver",
		BaseURL:   OpenAIDefaultBaseURL,
		Model:     OpenAIDefaultModel,
		APIKeyEnv: OpenAIDefaultAPIKeyEnv,
	}}
	defaults.SelectedProvider = OpenAIName
	defaults.CurrentModel = OpenAIDefaultModel

	manager := newTestManager(t, defaults)
	support := &mockDriverSupporter{supported: "other-driver"} // doesn't match missing-driver
	service := NewSelectionOrchestrator(manager, support, &mockModelCatalog{})

	if _, err := service.SelectProvider(context.Background(), OpenAIName); !errors.Is(err, ErrDriverNotSupported) {
		t.Fatalf("expected SelectProvider() to reject unsupported driver, got %v", err)
	}
}

func TestResolveCurrentModelHelper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		currentModel string
		models       []ModelDescriptor
		fallback     string
		expected     string
		changed      bool
	}{
		{
			name:         "current model in list",
			currentModel: "gpt-4o",
			models: []ModelDescriptor{
				{ID: "gpt-4.1"},
				{ID: "gpt-4o"},
				{ID: "gpt-5.4"},
			},
			fallback: "gpt-4.1",
			expected: "gpt-4o",
			changed:  false,
		},
		{
			name:         "current model falls back to provider default",
			currentModel: "unknown-model",
			models: []ModelDescriptor{
				{ID: "gpt-4.1"},
				{ID: "gpt-4o"},
				{ID: "gpt-5.4"},
			},
			fallback: "gpt-4.1",
			expected: "gpt-4.1",
			changed:  true,
		},
		{
			name:         "missing fallback keeps current model unchanged",
			currentModel: "unknown-model",
			models: []ModelDescriptor{
				{ID: "gpt-4o"},
			},
			fallback: "gpt-4.1",
			expected: "unknown-model",
			changed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, changed := ResolveCurrentModel(tt.currentModel, tt.models, tt.fallback)
			if got != tt.expected || changed != tt.changed {
				t.Fatalf("ResolveCurrentModel() = (%q, %v), want (%q, %v)", got, changed, tt.expected, tt.changed)
			}
		})
	}
}

// --- SelectionService tests (pure config operations without provider dependency) ---

func TestSelectionService_SetCurrentModelEmptyModelID(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t, DefaultConfig())
	svc := NewSelectionService(manager)

	_, err := svc.SetCurrentModel(context.Background(), "", nil)
	if !errors.Is(err, ErrModelNotFound) {
		t.Fatalf("expected ErrModelNotFound for empty model ID, got %v", err)
	}

	_, err = svc.SetCurrentModel(context.Background(), "   ", nil)
	if !errors.Is(err, ErrModelNotFound) {
		t.Fatalf("expected ErrModelNotFound for whitespace model ID, got %v", err)
	}
}

func TestSelectionService_SetCurrentModelNotInList(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t, DefaultConfig())
	svc := NewSelectionService(manager)

	models := []ModelDescriptor{{ID: "gpt-4o"}}
	_, err := svc.SetCurrentModel(context.Background(), "missing-model", models)
	if !errors.Is(err, ErrModelNotFound) {
		t.Fatalf("expected ErrModelNotFound for model not in list, got %v", err)
	}
}

func TestSelectionService_EnsureSelectionNoChangeNeeded(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t, DefaultConfig())
	svc := NewSelectionService(manager)

	models := []ModelDescriptor{{ID: OpenAIDefaultModel}}
	_, changed, err := svc.EnsureSelection(context.Background(), models, "")
	if err != nil {
		t.Fatalf("EnsureSelection() error = %v", err)
	}
	if changed {
		t.Fatalf("expected no change when current model is valid")
	}
}

func TestContainsModelDescriptorID(t *testing.T) {
	t.Parallel()

	models := []ModelDescriptor{
		{ID: "GPT-4o"},
		{ID: "gpt-4-turbo"},
		{ID: "Claude-3"},
	}

	if !ContainsModelDescriptorID(models, "gpt-4o") {
		t.Fatalf("expected case-insensitive match")
	}
	if !ContainsModelDescriptorID(models, "GPT-4O") {
		t.Fatalf("expected case-insensitive match (upper)")
	}
	if ContainsModelDescriptorID(models, "gpt-3") {
		t.Fatalf("expected no match for missing model")
	}
	if ContainsModelDescriptorID(models, "") {
		t.Fatalf("expected no match for empty ID")
	}
	if ContainsModelDescriptorID(nil, "gpt-4o") {
		t.Fatalf("expected no match on nil slice")
	}
}

func newTestManager(t *testing.T, defaults *Config) *Manager {
	t.Helper()

	manager := NewManager(NewLoader(t.TempDir(), defaults))
	if _, err := manager.Load(context.Background()); err != nil {
		t.Fatalf("load config: %v", err)
	}
	return manager
}
