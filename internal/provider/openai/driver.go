package openai

import (
	"context"
	"net/http"

	"neo-code/internal/provider"
	"neo-code/internal/provider/transport"
	providertypes "neo-code/internal/provider/types"
)

// DriverName 是 builtin OpenAI provider 使用的驱动标识。
const DriverName = "openai"

// CompatibleDriverName 是 custom OpenAI-compatible provider 使用的驱动标识。
const CompatibleDriverName = "openaicompat"

// defaultRetryTransport 返回内置的带重试 HTTP Transport。
func defaultRetryTransport() http.RoundTripper {
	return transport.NewRetryTransport(http.DefaultTransport, transport.DefaultRetryConfig())
}

// Driver 返回 builtin OpenAI provider 的驱动定义。
func Driver() provider.DriverDefinition {
	return driverDefinition(DriverName)
}

// CompatibleDriver 返回 custom OpenAI-compatible provider 的驱动定义。
func CompatibleDriver() provider.DriverDefinition {
	return driverDefinition(CompatibleDriverName)
}

// driverDefinition 根据驱动名构造共享的 OpenAI 协议驱动定义，避免 builtin 与 custom 路径分叉。
func driverDefinition(name string) provider.DriverDefinition {
	return provider.DriverDefinition{
		Name: name,
		Build: func(ctx context.Context, cfg provider.RuntimeConfig) (provider.Provider, error) {
			return New(cfg, withTransport(defaultRetryTransport()))
		},
		Discover: func(ctx context.Context, cfg provider.RuntimeConfig) ([]providertypes.ModelDescriptor, error) {
			p, err := New(cfg, withTransport(defaultRetryTransport()))
			if err != nil {
				return nil, err
			}
			return p.DiscoverModels(ctx)
		},
		Capabilities: provider.DriverTransportCapabilities{
			Streaming:           true,
			ToolTransport:       true,
			ModelDiscovery:      true,
			ImageInputTransport: false,
		},
	}
}
