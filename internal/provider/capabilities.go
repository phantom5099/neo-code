package provider

// DriverCapabilities 描述 driver 层本身是否能传输某类能力。
type DriverCapabilities struct {
	Streaming           bool
	ToolTransport       bool
	ModelDiscovery      bool
	ImageInputTransport bool
}
