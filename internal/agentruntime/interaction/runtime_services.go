package interaction

import (
	"context"
	"strings"

	"neo-code/internal/agentruntime"
	"neo-code/internal/agentruntime/memory"
	"neo-code/internal/config"
	"neo-code/internal/provider"
	"neo-code/internal/tool"
)

type ToolCall = tool.ToolCall
type ToolResult = tool.ToolResult
type ConfigMutationResult = agentruntime.ConfigMutationResult

type UISnapshot struct {
	ProviderName     string
	CurrentModel     string
	DefaultModel     string
	WorkspaceSummary string
}

const (
	TypeUserPreference = memory.TypeUserPreference
	TypeProjectRule    = memory.TypeProjectRule
	TypeCodeFact       = memory.TypeCodeFact
	TypeFixRecipe      = memory.TypeFixRecipe
	TypeSessionMemory  = memory.TypeSessionMemory
)

var (
	ErrInvalidAPIKey        = provider.ErrInvalidAPIKey
	ErrAPIKeyValidationSoft = provider.ErrAPIKeyValidationSoft
	ErrAPIKeyMissing        = agentruntime.ErrAPIKeyMissing
)

func ResolveWorkspaceRoot(workspaceFlag string) (string, error) {
	return tool.ResolveWorkspaceRoot(workspaceFlag)
}

func SetWorkspaceRoot(root string) error {
	return tool.SetWorkspaceRoot(root)
}

func GetWorkspaceRoot() string {
	return tool.GetWorkspaceRoot()
}

func RuntimeAPIKeyReady() bool {
	return strings.TrimSpace(config.RuntimeAPIKey()) != ""
}

func NormalizeToolParams(params map[string]interface{}) map[string]interface{} {
	return tool.NormalizeParams(params)
}

func ParseAssistantToolCalls(text string) []ToolCall {
	return agentruntime.ParseAssistantToolCalls(text)
}

func ExecuteToolCall(call ToolCall) *ToolResult {
	return agentruntime.ExecuteToolCall(call)
}

func ApproveToolCall(toolType, target string) {
	agentruntime.ApproveToolCall(toolType, target)
}

func InitializeSecurity(configDir string) error {
	return agentruntime.InitializeSecurity(configDir)
}

func UpdateAPIKeyEnvVar(ctx context.Context, configPath, envName string) (*ConfigMutationResult, error) {
	return agentruntime.UpdateAPIKeyEnvVar(ctx, configPath, envName)
}

func SwitchProvider(ctx context.Context, configPath, providerName string) (*ConfigMutationResult, error) {
	return agentruntime.SwitchProvider(ctx, configPath, providerName)
}

func SwitchModel(ctx context.Context, configPath, model string) (*ConfigMutationResult, error) {
	return agentruntime.SwitchModel(ctx, configPath, model)
}

func ValidateChatAPIKey(ctx context.Context, cfg *config.AppConfiguration) error {
	return provider.ValidateChatAPIKey(ctx, cfg)
}

func NormalizeProviderName(name string) (string, bool) {
	return provider.NormalizeProviderName(name)
}

func SupportedProviders() []string {
	return provider.SupportedProviders()
}

func DefaultModelForProvider(name string) string {
	return provider.DefaultModelForProvider(name)
}

func ReadUISnapshot(ctx context.Context, client ChatClient) UISnapshot {
	snapshot := UISnapshot{}
	if cfg := config.GlobalAppConfig; cfg != nil {
		snapshot.ProviderName = strings.TrimSpace(cfg.CurrentProviderName())
		snapshot.CurrentModel = strings.TrimSpace(cfg.CurrentModelName())
		if snapshot.ProviderName != "" {
			snapshot.DefaultModel = strings.TrimSpace(provider.DefaultModelForProvider(snapshot.ProviderName))
		}
	}
	if snapshot.CurrentModel == "" && client != nil {
		snapshot.CurrentModel = strings.TrimSpace(client.DefaultModel())
	}
	if snapshot.DefaultModel == "" {
		snapshot.DefaultModel = snapshot.CurrentModel
	}
	if summaryProvider, ok := client.(WorkingSessionSummaryProvider); ok && summaryProvider != nil {
		if summary, err := summaryProvider.GetWorkingSessionSummary(ctx); err == nil {
			snapshot.WorkspaceSummary = strings.TrimSpace(summary)
		}
	}
	return snapshot
}
