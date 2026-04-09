package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoaderLoadMissingConfigCreatesDefault(t *testing.T) {
	t.Parallel()

	loader := NewLoader(t.TempDir(), testDefaultConfig())
	if _, err := os.Stat(loader.ConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("expected config file to be missing before load, got %v", err)
	}

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg == nil {
		t.Fatalf("expected config to be created")
	}
	if _, err := os.Stat(loader.ConfigPath()); err != nil {
		t.Fatalf("expected config file to be created, got %v", err)
	}
}

func TestLoaderLoadMalformedYAML(t *testing.T) {
	t.Parallel()

	loader := NewLoader(t.TempDir(), testDefaultConfig())
	if err := os.MkdirAll(loader.BaseDir(), 0o755); err != nil {
		t.Fatalf("mkdir base dir: %v", err)
	}
	if err := os.WriteFile(loader.ConfigPath(), []byte("providers:\n  - name: [\n"), 0o644); err != nil {
		t.Fatalf("write malformed config: %v", err)
	}

	_, err := loader.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "parse config file") {
		t.Fatalf("expected malformed yaml parse error, got %v", err)
	}
}

func TestLoaderRejectsLegacyWorkdirKey(t *testing.T) {
	t.Parallel()

	loader := NewLoader(t.TempDir(), testDefaultConfig())
	if err := os.MkdirAll(loader.BaseDir(), 0o755); err != nil {
		t.Fatalf("mkdir base dir: %v", err)
	}
	raw := `
selected_provider: openai
current_model: gpt-4.1
workdir: .
shell: powershell
`
	if err := os.WriteFile(loader.ConfigPath(), []byte(strings.TrimSpace(raw)+"\n"), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	_, err := loader.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "legacy config key \"workdir\" is no longer supported") {
		t.Fatalf("expected legacy workdir rejection, got %v", err)
	}
}

func TestLoaderRejectsLegacyDefaultWorkdirKey(t *testing.T) {
	t.Parallel()

	loader := NewLoader(t.TempDir(), testDefaultConfig())
	if err := os.MkdirAll(loader.BaseDir(), 0o755); err != nil {
		t.Fatalf("mkdir base dir: %v", err)
	}
	raw := `
selected_provider: openai
current_model: gpt-4.1
default_workdir: .
shell: powershell
`
	if err := os.WriteFile(loader.ConfigPath(), []byte(strings.TrimSpace(raw)+"\n"), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	_, err := loader.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "legacy config key \"default_workdir\" is no longer supported") {
		t.Fatalf("expected legacy default_workdir rejection, got %v", err)
	}
}

func TestLoaderLoadInvalidBaseDir(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	baseFile := filepath.Join(tempDir, "not-a-directory")
	if err := os.WriteFile(baseFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write base file: %v", err)
	}

	loader := NewLoader(baseFile, testDefaultConfig())
	_, err := loader.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "create config dir") {
		t.Fatalf("expected invalid base dir error, got %v", err)
	}
}

func TestLoaderRewritesLegacyProvidersFormatOnLoad(t *testing.T) {
	t.Parallel()

	loader := NewLoader(t.TempDir(), testDefaultConfig())
	if err := os.MkdirAll(loader.BaseDir(), 0o755); err != nil {
		t.Fatalf("mkdir base dir: %v", err)
	}

	legacy := `
selected_provider: openai
current_model: gpt-5.4
shell: powershell
providers:
  - name: openai
    type: openai
    base_url: https://example.com/v1
    model: gpt-5.4
    api_key_env: OPENAI_API_KEY
`
	if err := os.WriteFile(loader.ConfigPath(), []byte(strings.TrimSpace(legacy)+"\n"), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	provider, err := cfg.SelectedProviderConfig()
	if err != nil {
		t.Fatalf("SelectedProviderConfig() error = %v", err)
	}
	if provider.BaseURL != testBaseURL {
		t.Fatalf("expected builtin provider base url %q, got %q", testBaseURL, provider.BaseURL)
	}
	if cfg.CurrentModel != "gpt-5.4" {
		t.Fatalf("expected current model to stay %q, got %q", "gpt-5.4", cfg.CurrentModel)
	}

	rewritten, err := os.ReadFile(loader.ConfigPath())
	if err != nil {
		t.Fatalf("read rewritten config: %v", err)
	}
	text := string(rewritten)
	if strings.Contains(text, "default_workdir:") || strings.Contains(text, "\nworkdir:") || strings.HasPrefix(text, "workdir:") {
		t.Fatalf("expected rewritten config to avoid any workdir keys, got:\n%s", text)
	}
	if strings.Contains(text, "provider_overrides:") {
		t.Fatalf("expected rewritten config to drop provider overrides, got:\n%s", text)
	}
	if strings.Contains(text, "\nproviders:") || strings.HasPrefix(text, "providers:") {
		t.Fatalf("expected rewritten config to omit providers list, got:\n%s", text)
	}
	if strings.Contains(text, "models:") || strings.Contains(text, "base_url:") || strings.Contains(text, "api_key_env:") {
		t.Fatalf("expected rewritten config to omit provider metadata, got:\n%s", text)
	}
}

func TestLoaderRewritesNormalizedSelectionStateOnLoad(t *testing.T) {
	t.Parallel()

	loader := NewLoader(t.TempDir(), testDefaultConfig())
	if err := os.MkdirAll(loader.BaseDir(), 0o755); err != nil {
		t.Fatalf("mkdir base dir: %v", err)
	}

	raw := `
selected_provider: missing-provider
shell: powershell
`
	if err := os.WriteFile(loader.ConfigPath(), []byte(strings.TrimSpace(raw)+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SelectedProvider != testProviderName {
		t.Fatalf("expected selected provider %q, got %q", testProviderName, cfg.SelectedProvider)
	}
	if cfg.CurrentModel != testModel {
		t.Fatalf("expected current model %q, got %q", testModel, cfg.CurrentModel)
	}

	rewritten, err := os.ReadFile(loader.ConfigPath())
	if err != nil {
		t.Fatalf("read rewritten config: %v", err)
	}
	text := string(rewritten)
	if !strings.Contains(text, "selected_provider: "+testProviderName) {
		t.Fatalf("expected rewritten config to persist selected provider, got:\n%s", text)
	}
	if !strings.Contains(text, "current_model: "+testModel) {
		t.Fatalf("expected rewritten config to persist current model, got:\n%s", text)
	}
}

func TestLoaderRewritesMissingCurrentModelOnLoad(t *testing.T) {
	t.Parallel()

	loader := NewLoader(t.TempDir(), testDefaultConfig())
	if err := os.MkdirAll(loader.BaseDir(), 0o755); err != nil {
		t.Fatalf("mkdir base dir: %v", err)
	}

	raw := `
selected_provider: openai
shell: powershell
`
	if err := os.WriteFile(loader.ConfigPath(), []byte(strings.TrimSpace(raw)+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SelectedProvider != testProviderName {
		t.Fatalf("expected selected provider %q, got %q", testProviderName, cfg.SelectedProvider)
	}
	if cfg.CurrentModel != testModel {
		t.Fatalf("expected current model %q, got %q", testModel, cfg.CurrentModel)
	}

	rewritten, err := os.ReadFile(loader.ConfigPath())
	if err != nil {
		t.Fatalf("read rewritten config: %v", err)
	}
	text := string(rewritten)
	if !strings.Contains(text, "current_model: "+testModel) {
		t.Fatalf("expected rewritten config to persist current model, got:\n%s", text)
	}
}

func TestLoaderLoadsCustomProvidersFromProvidersDirectory(t *testing.T) {
	t.Parallel()

	loader := NewLoader(t.TempDir(), testDefaultConfig())
	if err := os.MkdirAll(filepath.Join(loader.BaseDir(), providersDirName, "company-gateway"), 0o755); err != nil {
		t.Fatalf("mkdir custom provider dir: %v", err)
	}

	rawConfig := `
selected_provider: company-gateway
current_model: deepseek-coder
shell: powershell
`
	if err := os.WriteFile(loader.ConfigPath(), []byte(strings.TrimSpace(rawConfig)+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	providerYAML := `
name: company-gateway
driver: openaicompat
api_key_env: COMPANY_GATEWAY_API_KEY
openai_compatible:
  profile: generic
  base_url: https://llm.example.com/v1
  api_style: chat_completions
`
	customDir := filepath.Join(loader.BaseDir(), providersDirName, "company-gateway")
	if err := os.WriteFile(filepath.Join(customDir, customProviderConfigName), []byte(strings.TrimSpace(providerYAML)+"\n"), 0o644); err != nil {
		t.Fatalf("write provider.yaml: %v", err)
	}

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SelectedProvider != "company-gateway" {
		t.Fatalf("expected selected provider company-gateway, got %q", cfg.SelectedProvider)
	}
	if cfg.CurrentModel != "deepseek-coder" {
		t.Fatalf("expected current model deepseek-coder, got %q", cfg.CurrentModel)
	}

	customProvider, err := cfg.ProviderByName("company-gateway")
	if err != nil {
		t.Fatalf("ProviderByName(company-gateway) error = %v", err)
	}
	if customProvider.Source != ProviderSourceCustom {
		t.Fatalf("expected custom provider source, got %+v", customProvider)
	}
	if customProvider.Driver != "openaicompat" {
		t.Fatalf("expected custom provider driver openaicompat, got %q", customProvider.Driver)
	}
	if customProvider.APIStyle != "chat_completions" {
		t.Fatalf("expected api_style chat_completions, got %q", customProvider.APIStyle)
	}
	if customProvider.BaseURL != "https://llm.example.com/v1" {
		t.Fatalf("expected base url https://llm.example.com/v1, got %q", customProvider.BaseURL)
	}
	if customProvider.Model != "" {
		t.Fatalf("expected custom provider default model to be empty, got %q", customProvider.Model)
	}
	if len(customProvider.Models) != 0 {
		t.Fatalf("expected custom provider models to come only from remote discovery, got %+v", customProvider.Models)
	}
}

func TestLoaderRejectsCustomProviderDefaultModel(t *testing.T) {
	t.Parallel()

	loader := NewLoader(t.TempDir(), testDefaultConfig())
	customDir := filepath.Join(loader.BaseDir(), providersDirName, "company-gateway")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatalf("mkdir custom provider dir: %v", err)
	}

	providerYAML := `
name: company-gateway
driver: openaicompat
default_model: deepseek-coder
api_key_env: COMPANY_GATEWAY_API_KEY
openai_compatible:
  base_url: https://llm.example.com/v1
  api_style: chat_completions
`
	if err := os.WriteFile(filepath.Join(customDir, customProviderConfigName), []byte(strings.TrimSpace(providerYAML)+"\n"), 0o644); err != nil {
		t.Fatalf("write provider.yaml: %v", err)
	}

	_, err := loader.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "does not support default_model") {
		t.Fatalf("expected default_model rejection, got %v", err)
	}
}

func TestLoaderIgnoresCustomProviderModelsYAML(t *testing.T) {
	t.Parallel()

	loader := NewLoader(t.TempDir(), testDefaultConfig())
	customDir := filepath.Join(loader.BaseDir(), providersDirName, "company-gateway")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatalf("mkdir custom provider dir: %v", err)
	}

	providerYAML := `
name: company-gateway
driver: openaicompat
api_key_env: COMPANY_GATEWAY_API_KEY
openai_compatible:
  base_url: https://llm.example.com/v1
  api_style: chat_completions
`
	modelsYAML := `
models:
  - name: deepseek-coder
`
	if err := os.WriteFile(filepath.Join(customDir, customProviderConfigName), []byte(strings.TrimSpace(providerYAML)+"\n"), 0o644); err != nil {
		t.Fatalf("write provider.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(customDir, "models.yaml"), []byte(strings.TrimSpace(modelsYAML)+"\n"), 0o644); err != nil {
		t.Fatalf("write models.yaml: %v", err)
	}

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	customProvider, err := cfg.ProviderByName("company-gateway")
	if err != nil {
		t.Fatalf("ProviderByName(company-gateway) error = %v", err)
	}
	if len(customProvider.Models) != 0 {
		t.Fatalf("expected models.yaml to be ignored, got %+v", customProvider.Models)
	}
}

func TestLoaderRejectsCustomProviderNameConflictingWithBuiltin(t *testing.T) {
	t.Parallel()

	loader := NewLoader(t.TempDir(), testDefaultConfig())
	customDir := filepath.Join(loader.BaseDir(), providersDirName, "openai")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatalf("mkdir custom provider dir: %v", err)
	}

	providerYAML := `
name: openai
driver: openaicompat
api_key_env: OPENAI_GATEWAY_API_KEY
openai_compatible:
  base_url: https://api.example.com/v1
  api_style: chat_completions
`
	if err := os.WriteFile(filepath.Join(customDir, customProviderConfigName), []byte(strings.TrimSpace(providerYAML)+"\n"), 0o644); err != nil {
		t.Fatalf("write provider.yaml: %v", err)
	}

	_, err := loader.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "duplicate provider name") {
		t.Fatalf("expected duplicate provider name error, got %v", err)
	}
}

func TestLoaderRejectsDuplicateCustomProviderEndpointIdentity(t *testing.T) {
	t.Parallel()

	loader := NewLoader(t.TempDir(), testDefaultConfig())
	customA := filepath.Join(loader.BaseDir(), providersDirName, "gateway-a")
	customB := filepath.Join(loader.BaseDir(), providersDirName, "gateway-b")
	for _, dir := range []string{customA, customB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir custom provider dir: %v", err)
		}
	}

	providerA := `
name: gateway-a
driver: openaicompat
api_key_env: GATEWAY_A_API_KEY
openai_compatible:
  base_url: https://api.example.com/v1/
  api_style: responses
`
	providerB := `
name: gateway-b
driver: openaicompat
api_key_env: GATEWAY_B_API_KEY
openai_compatible:
  base_url: https://API.EXAMPLE.COM/v1
  api_style: Responses
`
	if err := os.WriteFile(filepath.Join(customA, customProviderConfigName), []byte(strings.TrimSpace(providerA)+"\n"), 0o644); err != nil {
		t.Fatalf("write provider a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(customB, customProviderConfigName), []byte(strings.TrimSpace(providerB)+"\n"), 0o644); err != nil {
		t.Fatalf("write provider b: %v", err)
	}

	_, err := loader.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "duplicate provider endpoint") {
		t.Fatalf("expected duplicate provider endpoint error, got %v", err)
	}
}

func TestLoaderRejectsCustomProviderLegacyOpenAIDriver(t *testing.T) {
	t.Parallel()

	loader := NewLoader(t.TempDir(), testDefaultConfig())
	customDir := filepath.Join(loader.BaseDir(), providersDirName, "legacy-gateway")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatalf("mkdir custom provider dir: %v", err)
	}

	providerYAML := `
name: legacy-gateway
driver: openai
api_key_env: LEGACY_GATEWAY_API_KEY
base_url: https://legacy.example.com/v1
`
	if err := os.WriteFile(filepath.Join(customDir, customProviderConfigName), []byte(strings.TrimSpace(providerYAML)+"\n"), 0o644); err != nil {
		t.Fatalf("write provider.yaml: %v", err)
	}

	_, err := loader.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no longer supported") {
		t.Fatalf("expected legacy driver rejection, got %v", err)
	}
}
