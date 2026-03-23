package configs

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultAPIKeyEnvVar = "AI_API_KEY"

type AppConfiguration struct {
	App struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	} `yaml:"app"`

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
		ShortTermTurns int `yaml:"short_term_turns"`
	} `yaml:"history"`

	Persona struct {
		FilePath string `yaml:"file_path"`
	} `yaml:"persona"`
}

var GlobalAppConfig *AppConfiguration

// DefaultAppConfig 返回内置的应用默认配置。
func DefaultAppConfig() *AppConfiguration {
	cfg := &AppConfiguration{}
	cfg.App.Name = "NeoCode"
	cfg.App.Version = "1.0.0"
	cfg.AI.Provider = "openll"
	cfg.AI.APIKey = DefaultAPIKeyEnvVar
	cfg.AI.Model = "gpt-5.4"
	cfg.Memory.TopK = 5
	cfg.Memory.MinMatchScore = 2.2
	cfg.Memory.MaxPromptChars = 1800
	cfg.Memory.MaxItems = 1000
	cfg.Memory.StoragePath = "./data/memory_rules.json"
	cfg.Memory.PersistTypes = []string{"user_preference", "project_rule", "code_fact", "fix_recipe"}
	cfg.History.ShortTermTurns = 6
	cfg.Persona.FilePath = DefaultPersonaFilePath
	return cfg
}

// LoadAppConfig 加载运行时配置并保存到全局变量。
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

// LoadBootstrapConfig 加载不依赖运行时密钥的基础配置。
func LoadBootstrapConfig(filePath string) (*AppConfiguration, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件时出错: %w", err)
	}

	cfg := DefaultAppConfig()
	//解析data数据覆盖到cfg上
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析yaml信息失败: %w", err)
	}
	if err := cfg.ValidateBase(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// EnsureConfigFile 加载已有配置文件，或在缺失时写入默认配置。
func EnsureConfigFile(filePath string) (*AppConfiguration, bool, error) {
	if _, err := os.Stat(filePath); err == nil {
		cfg, loadErr := LoadBootstrapConfig(filePath)
		return cfg, false, loadErr
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, false, fmt.Errorf("文件不存在: %w", err)
	}

	cfg := DefaultAppConfig()
	if err := WriteAppConfig(filePath, cfg); err != nil {
		return nil, false, err
	}
	return cfg, true, nil
}

// WriteAppConfig 将应用配置写入磁盘。
func WriteAppConfig(filePath string, cfg *AppConfiguration) error {
	if cfg == nil {
		return fmt.Errorf("配置信息为空")
	}
	cfgCopy := *cfg
	cfgCopy.AI.APIKey = strings.TrimSpace(cfgCopy.AI.APIKey)
	data, err := yaml.Marshal(&cfgCopy)
	if err != nil {
		return fmt.Errorf("序列化yaml信息时错误: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("向yaml文件写入配置信息时错误: %w", err)
	}
	return nil
}

// Validate 检查配置是否满足运行时要求。
func (c *AppConfiguration) Validate() error {
	return c.ValidateRuntime()
}

// ValidateBase 检查不包含密钥的基础配置是否合法。
func (c *AppConfiguration) ValidateBase() error {
	if c == nil {
		return fmt.Errorf("app config is nil")
	}
	providerName := normalizeProviderName(c.AI.Provider)
	if providerName == "" {
		return fmt.Errorf("invalid config: ai.provider is required")
	}
	if !isSupportedProvider(providerName) {
		return fmt.Errorf("invalid config: unsupported ai.provider %q", strings.TrimSpace(c.AI.Provider))
	}
	c.AI.Provider = providerName
	if strings.TrimSpace(c.AI.Model) == "" {
		return fmt.Errorf("invalid config: ai.model is required")
	}
	if c.Memory.TopK <= 0 {
		return fmt.Errorf("invalid config: memory.top_k must be greater than 0")
	}
	if c.Memory.MinMatchScore < 0 {
		return fmt.Errorf("invalid config: memory.min_match_score must not be negative")
	}
	if c.Memory.MaxPromptChars <= 0 {
		return fmt.Errorf("invalid config: memory.max_prompt_chars must be greater than 0")
	}
	if c.Memory.MaxItems <= 0 {
		return fmt.Errorf("invalid config: memory.max_items must be greater than 0")
	}
	if strings.TrimSpace(c.Memory.StoragePath) == "" {
		return fmt.Errorf("invalid config: memory.storage_path is required")
	}
	if c.History.ShortTermTurns <= 0 {
		return fmt.Errorf("invalid config: history.short_term_turns must be greater than 0")
	}
	return nil
}

// ValidateRuntime 检查配置字段和运行时必需的环境变量。
func (c *AppConfiguration) ValidateRuntime() error {
	if err := c.ValidateBase(); err != nil {
		return err
	}
	envVarName := c.APIKeyEnvVarName()
	if c.RuntimeAPIKey() == "" {
		return fmt.Errorf("invalid runtime: %s environment variable is required", envVarName)
	}
	return nil
}

// APIKeyEnvVarName 返回当前配置使用的 API Key 环境变量名。
func (c *AppConfiguration) APIKeyEnvVarName() string {
	if c == nil {
		return DefaultAPIKeyEnvVar
	}
	if name := strings.TrimSpace(c.AI.APIKey); name != "" {
		return name
	}
	return DefaultAPIKeyEnvVar
}

// RuntimeAPIKey 返回配置指向的环境变量中的 API Key，并去掉首尾空白。
func (c *AppConfiguration) RuntimeAPIKey() string {
	return strings.TrimSpace(os.Getenv(c.APIKeyEnvVarName()))
}

// RuntimeAPIKeyEnvVarName 返回全局配置当前使用的 API Key 环境变量名。
func RuntimeAPIKeyEnvVarName() string {
	if GlobalAppConfig != nil {
		return GlobalAppConfig.APIKeyEnvVarName()
	}
	return DefaultAPIKeyEnvVar
}

// RuntimeAPIKey 返回全局配置指向的环境变量中的 API Key，并去掉首尾空白。
func RuntimeAPIKey() string {
	if GlobalAppConfig != nil {
		return GlobalAppConfig.RuntimeAPIKey()
	}
	return strings.TrimSpace(os.Getenv(DefaultAPIKeyEnvVar))
}

func normalizeProviderName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	if strings.EqualFold(trimmed, "modelscope") {
		return "modelscope"
	}
	if strings.EqualFold(trimmed, "deepseek") {
		return "deepseek"
	}
	if strings.EqualFold(trimmed, "openll") {
		return "openll"
	}
	if strings.EqualFold(trimmed, "siliconflow") {
		return "siliconflow"
	}
	if strings.EqualFold(trimmed, "openai") {
		return "openai"
	}
	if trimmed == "豆包大模型" {
		return "豆包大模型"
	}
	return trimmed
}

func isSupportedProvider(name string) bool {
	switch normalizeProviderName(name) {
	case "modelscope", "deepseek", "openll", "siliconflow", "豆包大模型", "openai":
		return true
	default:
		return false
	}
}
