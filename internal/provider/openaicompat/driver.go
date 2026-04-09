package openaicompat

import (
	"context"
	"net/http"

	"neo-code/internal/provider"
	"neo-code/internal/provider/transport"
	providertypes "neo-code/internal/provider/types"
)

// DriverName 是当前 OpenAI-compatible 协议驱动的唯一标识。
const DriverName = "openaicompat"

// defaultRetryTransport 返回内置的带重试 HTTP Transport。
func defaultRetryTransport() http.RoundTripper {
	return transport.NewRetryTransport(http.DefaultTransport, transport.DefaultRetryConfig())
}

// Driver 返回 OpenAI-compatible 协议驱动定义。
func Driver() provider.DriverDefinition {
	return driverDefinition(DriverName)
}

// driverDefinition 根据驱动名构造共享的 OpenAI-compatible 协议驱动定义。
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
