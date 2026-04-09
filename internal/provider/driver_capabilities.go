package provider

// DriverTransportCapabilities 描述 driver 层本身是否能传输某类运行时能力。
type DriverTransportCapabilities struct {
	Streaming           bool
	ToolTransport       bool
	ModelDiscovery      bool
	ImageInputTransport bool
}
