package config

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// ProviderIdentity identifies a concrete provider endpoint for discovery and caching.
type ProviderIdentity struct {
	Driver         string `json:"driver"`
	BaseURL        string `json:"base_url"`
	APIStyle       string `json:"api_style,omitempty"`
	DeploymentMode string `json:"deployment_mode,omitempty"`
	APIVersion     string `json:"api_version,omitempty"`
}

func (i ProviderIdentity) Key() string {
	parts := []string{i.Driver, i.BaseURL}
	if strings.TrimSpace(i.APIStyle) != "" {
		parts = append(parts, i.APIStyle)
	}
	if strings.TrimSpace(i.DeploymentMode) != "" {
		parts = append(parts, i.DeploymentMode)
	}
	if strings.TrimSpace(i.APIVersion) != "" {
		parts = append(parts, i.APIVersion)
	}
	return strings.Join(parts, "|")
}

// String 返回 provider 身份的可读字符串，便于错误信息与诊断日志复用。
func (i ProviderIdentity) String() string {
	return i.Key()
}

func NormalizeKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func NormalizeProviderName(name string) string {
	return NormalizeKey(name)
}

func NormalizeProviderDriver(driver string) string {
	return NormalizeKey(driver)
}

// NormalizeProviderAPIStyle 规范化 openaicompat 的 api_style，用于稳定生成连接身份。
func NormalizeProviderAPIStyle(apiStyle string) string {
	return NormalizeKey(apiStyle)
}

// NormalizeProviderDeploymentMode 规范化 Gemini deployment_mode，避免大小写与空白导致误判。
func NormalizeProviderDeploymentMode(mode string) string {
	return NormalizeKey(mode)
}

// NormalizeProviderAPIVersion 规范化 Anthropic api_version，用于稳定生成连接身份。
func NormalizeProviderAPIVersion(version string) string {
	return NormalizeKey(version)
}

func NormalizeProviderBaseURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("provider base_url is empty")
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("provider base_url %q is invalid: %w", raw, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("provider base_url %q must include scheme and host", raw)
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.User = nil

	if cleaned := path.Clean(strings.TrimSpace(parsed.Path)); cleaned == "." {
		parsed.Path = ""
	} else {
		parsed.Path = strings.TrimRight(cleaned, "/")
	}

	return parsed.String(), nil
}

func NewProviderIdentity(driver string, baseURL string) (ProviderIdentity, error) {
	normalizedDriver := NormalizeProviderDriver(driver)
	if normalizedDriver == "" {
		return ProviderIdentity{}, fmt.Errorf("provider driver is empty")
	}

	normalizedBaseURL, err := NormalizeProviderBaseURL(baseURL)
	if err != nil {
		return ProviderIdentity{}, err
	}

	return ProviderIdentity{
		Driver:  normalizedDriver,
		BaseURL: normalizedBaseURL,
	}, nil
}

// NormalizeProviderIdentity 根据 driver 规则规范化连接身份，保留参与缓存去重的专用字段。
func NormalizeProviderIdentity(identity ProviderIdentity) (ProviderIdentity, error) {
	normalizedDriver := NormalizeProviderDriver(identity.Driver)
	switch normalizedDriver {
	case "openaicompat":
		baseURL, err := NormalizeProviderBaseURL(identity.BaseURL)
		if err != nil {
			return ProviderIdentity{}, err
		}
		return ProviderIdentity{
			Driver:   normalizedDriver,
			BaseURL:  baseURL,
			APIStyle: NormalizeProviderAPIStyle(identity.APIStyle),
		}, nil
	case "gemini":
		baseURL, err := NormalizeProviderBaseURL(identity.BaseURL)
		if err != nil {
			return ProviderIdentity{}, err
		}
		return ProviderIdentity{
			Driver:         normalizedDriver,
			BaseURL:        baseURL,
			DeploymentMode: NormalizeProviderDeploymentMode(identity.DeploymentMode),
		}, nil
	case "anthropic":
		baseURL, err := NormalizeProviderBaseURL(identity.BaseURL)
		if err != nil {
			return ProviderIdentity{}, err
		}
		return ProviderIdentity{
			Driver:     normalizedDriver,
			BaseURL:    baseURL,
			APIVersion: NormalizeProviderAPIVersion(identity.APIVersion),
		}, nil
	default:
		return NewProviderIdentity(identity.Driver, identity.BaseURL)
	}
}

// NewProviderIdentityFromConfig 根据 provider 配置生成去重与缓存使用的规范化连接身份。
func NewProviderIdentityFromConfig(provider ProviderConfig) (ProviderIdentity, error) {
	return NormalizeProviderIdentity(ProviderIdentity{
		Driver:         provider.Driver,
		BaseURL:        provider.BaseURL,
		APIStyle:       provider.APIStyle,
		DeploymentMode: provider.DeploymentMode,
		APIVersion:     provider.APIVersion,
	})
}
