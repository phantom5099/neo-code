package provider

import (
	"fmt"
	"strings"

	"neo-code/internal/config"
)

type ProviderSpec struct {
	Name         string
	Protocol     string
	BaseURL      string
	DefaultModel string
	APIKeyEnv    string
}

func NormalizeProviderName(name string) (string, bool) {
	normalized := normalizeProviderName(name)
	return normalized, normalized != ""
}

func SupportedProviders() []string {
	providers := providerSpecs(config.GlobalAppConfig)
	names := make([]string, 0, len(providers))
	for _, spec := range providers {
		names = append(names, spec.Name)
	}
	return names
}

func DefaultModel() string {
	return DefaultModelForConfig(config.GlobalAppConfig)
}

func DefaultModelForConfig(cfg *config.AppConfiguration) string {
	if cfg != nil {
		if model := strings.TrimSpace(cfg.CurrentModelName()); model != "" {
			return model
		}
	}
	if spec, ok := providerSpecByName(cfg, providerNameFromConfig(cfg)); ok {
		return strings.TrimSpace(spec.DefaultModel)
	}
	return ""
}

func DefaultModelForProvider(name string) string {
	spec, ok := providerSpecByName(config.GlobalAppConfig, name)
	if !ok {
		spec, ok = providerSpecByName(nil, name)
	}
	if !ok {
		return ""
	}
	return strings.TrimSpace(spec.DefaultModel)
}

func CurrentProvider() string {
	return providerNameFromConfig(config.GlobalAppConfig)
}

func ResolveChatEndpoint(cfg *config.AppConfiguration, model string) (string, error) {
	spec, ok := providerSpecByName(cfg, providerNameFromConfig(cfg))
	if !ok {
		return "", fmt.Errorf("unsupported provider: %s", providerNameFromConfig(cfg))
	}
	if strings.TrimSpace(spec.BaseURL) == "" {
		return "", fmt.Errorf("provider %q does not have a configured chat URL", spec.Name)
	}
	resolvedModel := strings.TrimSpace(model)
	if resolvedModel == "" {
		resolvedModel = strings.TrimSpace(spec.DefaultModel)
	}
	return strings.ReplaceAll(strings.TrimSpace(spec.BaseURL), "{model}", resolvedModel), nil
}

func CurrentProviderProtocol(cfg *config.AppConfiguration) string {
	spec, ok := providerSpecByName(cfg, providerNameFromConfig(cfg))
	if !ok {
		return "openai"
	}
	if strings.TrimSpace(spec.Protocol) == "" {
		return "openai"
	}
	return strings.ToLower(strings.TrimSpace(spec.Protocol))
}

func providerNameFromConfig(cfg *config.AppConfiguration) string {
	if cfg != nil {
		if normalized, ok := NormalizeProviderName(cfg.CurrentProviderName()); ok {
			return normalized
		}
	}
	return "openai"
}

func providerSpecByName(cfg *config.AppConfiguration, name string) (ProviderSpec, bool) {
	normalized, ok := NormalizeProviderName(name)
	if !ok {
		return ProviderSpec{}, false
	}
	for _, spec := range providerSpecs(cfg) {
		if spec.Name == normalized {
			return spec, true
		}
	}
	return ProviderSpec{}, false
}

func providerSpecs(cfg *config.AppConfiguration) []ProviderSpec {
	source := config.DefaultProviderCatalog()
	if cfg != nil && len(cfg.Providers) > 0 {
		source = append([]config.ProviderProfile(nil), cfg.Providers...)
	}

	specs := make([]ProviderSpec, 0, len(source))
	for _, profile := range source {
		normalized := normalizeProviderName(profile.Name)
		if normalized == "" {
			continue
		}
		specs = append(specs, ProviderSpec{
			Name:         normalized,
			Protocol:     strings.ToLower(strings.TrimSpace(profile.Protocol)),
			BaseURL:      sanitizeBaseURL(profile.Protocol, profile.BaseURL),
			DefaultModel: strings.TrimSpace(profile.Model),
			APIKeyEnv:    strings.TrimSpace(profile.APIKeyEnv),
		})
	}
	return specs
}

func sanitizeBaseURL(protocol, raw string) string {
	baseURL := strings.TrimSpace(raw)
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "openai":
		return trimAfterKnownSuffix(baseURL, "/chat/completions")
	case "anthropic":
		return trimAfterKnownSuffix(baseURL, "/v1/messages")
	case "gemini":
		return trimAfterKnownSuffix(baseURL, "?alt=sse")
	default:
		return baseURL
	}
}

func trimAfterKnownSuffix(value, suffix string) string {
	if suffix == "" {
		return value
	}
	index := strings.Index(value, suffix)
	if index < 0 {
		return value
	}
	return value[:index+len(suffix)]
}

func normalizeProviderName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "modelscope":
		return "modelscope"
	case "deepseek":
		return "deepseek"
	case "openll":
		return "openll"
	case "siliconflow":
		return "siliconflow"
	case "doubao":
		return "doubao"
	case "openai":
		return "openai"
	case "anthropic":
		return "anthropic"
	case "gemini", "google":
		return "gemini"
	default:
		return ""
	}
}
