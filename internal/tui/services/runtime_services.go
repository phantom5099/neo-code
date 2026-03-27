package services

import (
	"context"

	"neo-code/internal/agentruntime/interaction"
	"neo-code/internal/config"
)

type ToolCall = interaction.ToolCall
type ToolResult = interaction.ToolResult
type ConfigMutationResult = interaction.ConfigMutationResult
type UISnapshot = interaction.UISnapshot

const (
	TypeUserPreference = interaction.TypeUserPreference
	TypeProjectRule    = interaction.TypeProjectRule
	TypeCodeFact       = interaction.TypeCodeFact
	TypeFixRecipe      = interaction.TypeFixRecipe
	TypeSessionMemory  = interaction.TypeSessionMemory
)

var (
	ErrInvalidAPIKey        = interaction.ErrInvalidAPIKey
	ErrAPIKeyValidationSoft = interaction.ErrAPIKeyValidationSoft
	ErrAPIKeyMissing        = interaction.ErrAPIKeyMissing
)

func ResolveWorkspaceRoot(workspaceFlag string) (string, error) {
	return interaction.ResolveWorkspaceRoot(workspaceFlag)
}

func SetWorkspaceRoot(root string) error {
	return interaction.SetWorkspaceRoot(root)
}

func GetWorkspaceRoot() string {
	return interaction.GetWorkspaceRoot()
}

func RuntimeAPIKeyReady() bool {
	return interaction.RuntimeAPIKeyReady()
}

func NormalizeToolParams(params map[string]interface{}) map[string]interface{} {
	return interaction.NormalizeToolParams(params)
}

func ParseAssistantToolCalls(text string) []ToolCall {
	return interaction.ParseAssistantToolCalls(text)
}

func ExecuteToolCall(call ToolCall) *ToolResult {
	return interaction.ExecuteToolCall(call)
}

func ApproveToolCall(toolType, target string) {
	interaction.ApproveToolCall(toolType, target)
}

func InitializeSecurity(configDir string) error {
	return interaction.InitializeSecurity(configDir)
}

func UpdateAPIKeyEnvVar(ctx context.Context, configPath, envName string) (*ConfigMutationResult, error) {
	return interaction.UpdateAPIKeyEnvVar(ctx, configPath, envName)
}

func SwitchProvider(ctx context.Context, configPath, providerName string) (*ConfigMutationResult, error) {
	return interaction.SwitchProvider(ctx, configPath, providerName)
}

func SwitchModel(ctx context.Context, configPath, model string) (*ConfigMutationResult, error) {
	return interaction.SwitchModel(ctx, configPath, model)
}

func ValidateChatAPIKey(ctx context.Context, cfg *config.AppConfiguration) error {
	return interaction.ValidateChatAPIKey(ctx, cfg)
}

func NormalizeProviderName(name string) (string, bool) {
	return interaction.NormalizeProviderName(name)
}

func SupportedProviders() []string {
	return interaction.SupportedProviders()
}

func DefaultModelForProvider(name string) string {
	return interaction.DefaultModelForProvider(name)
}

func ReadUISnapshot(ctx context.Context, client ChatClient) UISnapshot {
	return interaction.ReadUISnapshot(ctx, client)
}
