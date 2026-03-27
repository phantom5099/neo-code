package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultAPIKeyEnvVar = "AI_API_KEY"

type ProviderProfile struct {
	Name      string `yaml:"name"`
	Protocol  string `yaml:"protocol"`
	BaseURL   string `yaml:"url"`
	Model     string `yaml:"model_id"`
	APIKeyEnv string `yaml:"api_key_env"`
}

type AppConfiguration struct {
	App struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	} `yaml:"app"`

	Providers        []ProviderProfile `yaml:"providers"`
	SelectedProvider string            `yaml:"selected_provider"`
	CurrentModel     string            `yaml:"current_model"`

	AI struct {
		Provider string `yaml:"provider"`
		APIKey   string `yaml:"api_key"`
		Model    string `yaml:"model"`
	} `yaml:"ai"`

	Memory struct {
		TopK           int      `yaml:"top_k"`
		MinMatchScore  float64  `yaml:"min_match_score"`
		MaxPromptChars int      `yaml:"max_prompt_chars"`
		MaxItems       int      `yaml:"max_items"`
		StoragePath    string   `yaml:"storage_path"`
		PersistTypes   []string `yaml:"persist_types"`
	} `yaml:"memory"`

	History struct {
		ShortTermTurns           int    `yaml:"short_term_turns"`
		MaxToolContextMessages   int    `yaml:"max_tool_context_messages"`
		MaxToolContextOutputSize int    `yaml:"max_tool_context_output_size"`
		PersistSessionState      bool   `yaml:"persist_session_state"`
		WorkspaceStateDir        string `yaml:"workspace_state_dir"`
		ResumeLastSession        bool   `yaml:"resume_last_session"`
	} `yaml:"history"`

	Persona struct {
		FilePath string `yaml:"file_path"`
	} `yaml:"persona"`
}

var GlobalAppConfig *AppConfiguration

func DefaultProviderCatalog() []ProviderProfile {
	return []ProviderProfile{
		{Name: "openai", Protocol: "openai", BaseURL: "https://api.openai.com/v1/chat/completions", Model: "gpt-5.4", APIKeyEnv: DefaultAPIKeyEnvVar},
		{Name: "anthropic", Protocol: "anthropic", BaseURL: "https://api.anthropic.com/v1/messages", Model: "claude-sonnet-4-5", APIKeyEnv: "ANTHROPIC_API_KEY"},
		{Name: "gemini", Protocol: "gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta/models/{model}:streamGenerateContent?alt=sse", Model: "gemini-2.5-pro", APIKeyEnv: "GEMINI_API_KEY"},
		{Name: "deepseek", Protocol: "openai", BaseURL: "https://api.deepseek.com/chat/completions", Model: "deepseek-chat", APIKeyEnv: "DEEPSEEK_API_KEY"},
		{Name: "modelscope", Protocol: "openai", BaseURL: "https://api-inference.modelscope.cn/v1/chat/completions", Model: "Qwen/Qwen3-Coder-480B-A35B-Instruct", APIKeyEnv: "MODELSCOPE_API_KEY"},
		{Name: "siliconflow", Protocol: "openai", BaseURL: "https://api.siliconflow.cn/v1/chat/completions", Model: "zai-org/GLM-4.6", APIKeyEnv: "SILICONFLOW_API_KEY"},
		{Name: "doubao", Protocol: "openai", BaseURL: "https://ark.cn-beijing.volces.com/api/v3/chat/completions", Model: "doubao-pro-v1", APIKeyEnv: "DOUBAO_API_KEY"},
		{Name: "openll", Protocol: "openai", BaseURL: "https://www.openll.top/v1/chat/completions", Model: "gpt-5.4", APIKeyEnv: DefaultAPIKeyEnvVar},
	}
}

func DefaultAppConfig() *AppConfiguration {
	return DefaultAppConfigForPath(DefaultConfigPath())
}

func DefaultAppConfigForPath(filePath string) *AppConfiguration {
	cfg := &AppConfiguration{}
	cfg.App.Name = "NeoCode"
	cfg.App.Version = "1.0.0"
	cfg.Providers = DefaultProviderCatalog()
	cfg.SelectedProvider = "openll"
	cfg.CurrentModel = "gpt-5.4"
	configHome := configHomeDir(filePath)
	cfg.Memory.TopK = 5
	cfg.Memory.MinMatchScore = 2.2
	cfg.Memory.MaxPromptChars = 1800
	cfg.Memory.MaxItems = 1000
	cfg.Memory.StoragePath = filepath.Join(configHome, "data", "memory_rules.json")
	cfg.Memory.PersistTypes = []string{"user_preference", "project_rule", "code_fact", "fix_recipe"}
	cfg.History.ShortTermTurns = 6
	cfg.History.MaxToolContextMessages = 3
	cfg.History.MaxToolContextOutputSize = 4000
	cfg.History.PersistSessionState = true
	cfg.History.WorkspaceStateDir = filepath.Join(configHome, "data", "workspaces")
	cfg.History.ResumeLastSession = true
	cfg.Persona.FilePath = filepath.Join(configHome, "persona.txt")
	cfg.syncLegacyAIFields()
	return cfg
}

func LoadAppConfig(filePath string) error {
	cfg, err := LoadBootstrapConfig(filePath)
	if err != nil {
		return err
	}
	if err := cfg.ValidateRuntime(); err != nil {
		return err
	}
	GlobalAppConfig = cfg
	return nil
}

func LoadBootstrapConfig(filePath string) (*AppConfiguration, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := DefaultAppConfigForPath(filePath)
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}
	cfg.normalize()
	if err := cfg.ValidateBase(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func EnsureConfigFile(filePath string) (*AppConfiguration, bool, error) {
	if _, err := os.Stat(filePath); err == nil {
		cfg, loadErr := LoadBootstrapConfig(filePath)
		return cfg, false, loadErr
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, false, fmt.Errorf("stat config file: %w", err)
	}

	cfg := DefaultAppConfigForPath(filePath)
	if err := WriteAppConfig(filePath, cfg); err != nil {
		return nil, false, err
	}
	if err := ensureDefaultPersonaFile(cfg.Persona.FilePath); err != nil {
		return nil, false, err
	}
	return cfg, true, nil
}

func WriteAppConfig(filePath string, cfg *AppConfiguration) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	cfgCopy := *cfg
	cfgCopy.normalize()
	cfgCopy.syncLegacyAIFields()
	for i := range cfgCopy.Providers {
		cfgCopy.Providers[i].Name = strings.TrimSpace(cfgCopy.Providers[i].Name)
		cfgCopy.Providers[i].Protocol = strings.TrimSpace(cfgCopy.Providers[i].Protocol)
		cfgCopy.Providers[i].BaseURL = strings.TrimSpace(cfgCopy.Providers[i].BaseURL)
		cfgCopy.Providers[i].Model = strings.TrimSpace(cfgCopy.Providers[i].Model)
		cfgCopy.Providers[i].APIKeyEnv = strings.TrimSpace(cfgCopy.Providers[i].APIKeyEnv)
	}

	data, err := yaml.Marshal(&cfgCopy)
	if err != nil {
		return fmt.Errorf("marshal config yaml: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

func (c *AppConfiguration) Validate() error {
	return c.ValidateRuntime()
}

func (c *AppConfiguration) ValidateBase() error {
	if c == nil {
		return fmt.Errorf("config cannot be nil")
	}

	c.normalize()

	providerName := c.CurrentProviderName()
	if providerName == "" {
		return fmt.Errorf("invalid config: selected_provider is required")
	}
	spec, ok := c.ProviderByName(providerName)
	if !ok {
		return fmt.Errorf("invalid config: unsupported provider %q", providerName)
	}
	if strings.TrimSpace(spec.BaseURL) == "" {
		return fmt.Errorf("invalid config: provider %q url is required", providerName)
	}
	if strings.TrimSpace(c.CurrentModelName()) == "" {
		return fmt.Errorf("invalid config: current_model is required")
	}
	if c.Memory.TopK <= 0 {
		return fmt.Errorf("invalid config: memory.top_k must be > 0")
	}
	if c.Memory.MinMatchScore < 0 {
		return fmt.Errorf("invalid config: memory.min_match_score must be >= 0")
	}
	if c.Memory.MaxPromptChars <= 0 {
		return fmt.Errorf("invalid config: memory.max_prompt_chars must be > 0")
	}
	if c.Memory.MaxItems <= 0 {
		return fmt.Errorf("invalid config: memory.max_items must be > 0")
	}
	if strings.TrimSpace(c.Memory.StoragePath) == "" {
		return fmt.Errorf("invalid config: memory.storage_path is required")
	}
	if c.History.ShortTermTurns <= 0 {
		return fmt.Errorf("invalid config: history.short_term_turns must be > 0")
	}
	if c.History.MaxToolContextMessages < 0 {
		return fmt.Errorf("invalid config: history.max_tool_context_messages must be >= 0")
	}
	if c.History.MaxToolContextOutputSize <= 0 {
		return fmt.Errorf("invalid config: history.max_tool_context_output_size must be > 0")
	}
	if c.History.PersistSessionState && strings.TrimSpace(c.History.WorkspaceStateDir) == "" {
		return fmt.Errorf("invalid config: history.workspace_state_dir is required when session persistence is enabled")
	}
	return nil
}

func (c *AppConfiguration) ValidateRuntime() error {
	if err := c.ValidateBase(); err != nil {
		return err
	}
	envVarName := c.APIKeyEnvVarName()
	if c.RuntimeAPIKey() == "" {
		return fmt.Errorf("runtime invalid: environment variable %s is required", envVarName)
	}
	return nil
}

func (c *AppConfiguration) CurrentProviderName() string {
	if c == nil {
		return ""
	}
	if normalized := normalizeProviderName(c.AI.Provider); normalized != "" {
		return normalized
	}
	if normalized := normalizeProviderName(c.SelectedProvider); normalized != "" {
		return normalized
	}
	return ""
}

func (c *AppConfiguration) CurrentModelName() string {
	if c == nil {
		return ""
	}
	if model := strings.TrimSpace(c.AI.Model); model != "" {
		return model
	}
	if model := strings.TrimSpace(c.CurrentModel); model != "" {
		return model
	}
	if spec, ok := c.SelectedProviderProfile(); ok {
		return strings.TrimSpace(spec.Model)
	}
	return ""
}

func (c *AppConfiguration) ProviderByName(name string) (ProviderProfile, bool) {
	if c == nil {
		return ProviderProfile{}, false
	}
	normalized := normalizeProviderName(name)
	if normalized == "" {
		return ProviderProfile{}, false
	}
	for _, provider := range c.Providers {
		if normalizeProviderName(provider.Name) == normalized {
			provider.Name = normalized
			return provider, true
		}
	}
	return ProviderProfile{}, false
}

func (c *AppConfiguration) SelectedProviderProfile() (ProviderProfile, bool) {
	return c.ProviderByName(c.CurrentProviderName())
}

func (c *AppConfiguration) SetSelectedProvider(name string) error {
	if c == nil {
		return fmt.Errorf("config cannot be nil")
	}
	spec, ok := c.ProviderByName(name)
	if !ok {
		return fmt.Errorf("unsupported provider: %s", name)
	}
	oldProvider := c.CurrentProviderName()
	c.SelectedProvider = spec.Name
	c.AI.Provider = spec.Name
	if strings.TrimSpace(c.CurrentModel) == "" || oldProvider != spec.Name {
		c.CurrentModel = strings.TrimSpace(spec.Model)
		c.AI.Model = strings.TrimSpace(spec.Model)
	}
	c.syncLegacyAIFields()
	return nil
}

func (c *AppConfiguration) SetCurrentModel(model string) {
	if c == nil {
		return
	}
	c.CurrentModel = strings.TrimSpace(model)
	c.AI.Model = strings.TrimSpace(model)
	c.syncLegacyAIFields()
}

func (c *AppConfiguration) SetAPIKeyEnvVarName(envName string) {
	if c == nil {
		return
	}
	envName = strings.TrimSpace(envName)
	if spec, ok := c.SelectedProviderProfile(); ok {
		for i := range c.Providers {
			if normalizeProviderName(c.Providers[i].Name) == spec.Name {
				c.Providers[i].APIKeyEnv = envName
			}
		}
	}
	c.AI.APIKey = envName
	c.syncLegacyAIFields()
}

func (c *AppConfiguration) APIKeyEnvVarName() string {
	if c == nil {
		return DefaultAPIKeyEnvVar
	}
	if name := strings.TrimSpace(c.AI.APIKey); name != "" {
		return name
	}
	if spec, ok := c.SelectedProviderProfile(); ok {
		if name := strings.TrimSpace(spec.APIKeyEnv); name != "" {
			return name
		}
	}
	return DefaultAPIKeyEnvVar
}

func (c *AppConfiguration) RuntimeAPIKey() string {
	return strings.TrimSpace(os.Getenv(c.APIKeyEnvVarName()))
}

func RuntimeAPIKeyEnvVarName() string {
	if GlobalAppConfig != nil {
		return GlobalAppConfig.APIKeyEnvVarName()
	}
	return DefaultAPIKeyEnvVar
}

func RuntimeAPIKey() string {
	if GlobalAppConfig != nil {
		return GlobalAppConfig.RuntimeAPIKey()
	}
	return strings.TrimSpace(os.Getenv(DefaultAPIKeyEnvVar))
}

func (c *AppConfiguration) normalize() {
	if c == nil {
		return
	}
	if len(c.Providers) == 0 {
		c.Providers = DefaultProviderCatalog()
	}
	if normalized := normalizeProviderName(c.SelectedProvider); normalized != "" {
		c.SelectedProvider = normalized
	} else if normalized := normalizeProviderName(c.AI.Provider); normalized != "" {
		c.SelectedProvider = normalized
	}
	if strings.TrimSpace(c.SelectedProvider) == "" && len(c.Providers) > 0 {
		c.SelectedProvider = normalizeProviderName(c.Providers[0].Name)
	}
	if strings.TrimSpace(c.CurrentModel) == "" {
		c.CurrentModel = strings.TrimSpace(c.AI.Model)
	}
	if strings.TrimSpace(c.CurrentModel) == "" {
		if spec, ok := c.SelectedProviderProfile(); ok {
			c.CurrentModel = strings.TrimSpace(spec.Model)
		}
	}
	if strings.TrimSpace(c.Memory.StoragePath) == "" {
		c.Memory.StoragePath = DefaultMemoryStoragePath()
	}
	if strings.TrimSpace(c.History.WorkspaceStateDir) == "" {
		c.History.WorkspaceStateDir = DefaultWorkspaceStateDir()
	}
	if strings.TrimSpace(c.Persona.FilePath) == "" {
		c.Persona.FilePath = DefaultPersonaFilePath
	}
	if apiKeyEnv := strings.TrimSpace(c.AI.APIKey); apiKeyEnv != "" {
		if spec, ok := c.SelectedProviderProfile(); ok {
			for i := range c.Providers {
				if normalizeProviderName(c.Providers[i].Name) == spec.Name {
					c.Providers[i].APIKeyEnv = apiKeyEnv
				}
			}
		}
	}
	c.syncLegacyAIFields()
}

func (c *AppConfiguration) syncLegacyAIFields() {
	if c == nil {
		return
	}
	c.AI.Provider = c.CurrentProviderName()
	c.AI.Model = c.CurrentModelName()
	c.AI.APIKey = c.APIKeyEnvVarName()
}

func normalizeProviderName(name string) string {
	trimmed := strings.TrimSpace(strings.ToLower(name))
	switch trimmed {
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
		return strings.TrimSpace(name)
	}
}

func configHomeDir(filePath string) string {
	trimmed := strings.TrimSpace(filePath)
	if trimmed == "" {
		return AppHomeDir()
	}

	absPath, err := filepath.Abs(trimmed)
	if err == nil {
		return filepath.Dir(absPath)
	}
	return filepath.Dir(trimmed)
}
