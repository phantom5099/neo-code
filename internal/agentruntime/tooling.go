package agentruntime

import (
	"neo-code/internal/tool"
	toolprotocol "neo-code/internal/tool/protocol"
	toolregistry "neo-code/internal/tool/registry"
)

func ParseAssistantToolCalls(text string) []tool.ToolCall {
	return toolprotocol.ParseAssistantToolCalls(text)
}

func ExecuteToolCall(call tool.ToolCall) *tool.ToolResult {
	return toolregistry.GlobalRegistry.Execute(call)
}

func ApproveToolCall(toolType, target string) {
	tool.ApproveSecurityAsk(toolType, target)
}

func InitializeSecurity(configDir string) error {
	return tool.InitializeSecurity(configDir)
}
