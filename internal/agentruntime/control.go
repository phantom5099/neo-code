package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"neo-code/internal/config"
	"neo-code/internal/provider"
)

var ErrAPIKeyMissing = errors.New("API key environment variable is not set")

type ConfigMutationResult struct {
	Provider      string
	Model         string
	APIKeyEnvVar  string
	APIKeyReady   bool
	ValidationErr error
}

func UpdateAPIKeyEnvVar(ctx context.Context, configPath, envName string) (*ConfigMutationResult, error) {
	cfg := config.GlobalAppConfig
	if cfg == nil {
		return nil, fmt.Errorf("app config is not loaded")
	}

	previous := cfg.APIKeyEnvVarName()
	cfg.SetAPIKeyEnvVarName(envName)
	if strings.TrimSpace(cfg.APIKeyEnvVarName()) == "" {
		cfg.SetAPIKeyEnvVarName(previous)
		return nil, fmt.Errorf("api key environment variable name cannot be empty")
	}
	if err := config.WriteAppConfig(configPath, cfg); err != nil {
		cfg.SetAPIKeyEnvVarName(previous)
		return nil, err
	}

	return validateConfigState(ctx, cfg), nil
}

func SwitchProvider(ctx context.Context, configPath, providerName string) (*ConfigMutationResult, error) {
	cfg := config.GlobalAppConfig
	if cfg == nil {
		return nil, fmt.Errorf("app config is not loaded")
	}

	normalized, ok := provider.NormalizeProviderName(providerName)
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}

	previousProvider := cfg.CurrentProviderName()
	previousModel := cfg.CurrentModelName()
	if err := cfg.SetSelectedProvider(normalized); err != nil {
		return nil, err
	}
	if err := config.WriteAppConfig(configPath, cfg); err != nil {
		_ = cfg.SetSelectedProvider(previousProvider)
		cfg.SetCurrentModel(previousModel)
		return nil, err
	}

	return validateConfigState(ctx, cfg), nil
}

func SwitchModel(ctx context.Context, configPath, model string) (*ConfigMutationResult, error) {
	cfg := config.GlobalAppConfig
	if cfg == nil {
		return nil, fmt.Errorf("app config is not loaded")
	}

	previousModel := cfg.CurrentModelName()
	cfg.SetCurrentModel(strings.TrimSpace(model))
	if cfg.CurrentModelName() == "" {
		cfg.SetCurrentModel(previousModel)
		return nil, fmt.Errorf("model cannot be empty")
	}
	if err := config.WriteAppConfig(configPath, cfg); err != nil {
		cfg.SetCurrentModel(previousModel)
		return nil, err
	}

	return validateConfigState(ctx, cfg), nil
}

func validateConfigState(ctx context.Context, cfg *config.AppConfiguration) *ConfigMutationResult {
	result := &ConfigMutationResult{
		Provider:     cfg.CurrentProviderName(),
		Model:        cfg.CurrentModelName(),
		APIKeyEnvVar: cfg.APIKeyEnvVarName(),
	}

	if strings.TrimSpace(cfg.RuntimeAPIKey()) == "" {
		result.ValidationErr = fmt.Errorf("%w: %s", ErrAPIKeyMissing, result.APIKeyEnvVar)
		return result
	}

	if err := provider.ValidateChatAPIKey(ctx, cfg); err != nil {
		result.ValidationErr = err
		return result
	}

	result.APIKeyReady = true
	return result
}
