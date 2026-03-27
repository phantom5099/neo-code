package interaction

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type MutationErrorKind string

const (
	MutationErrorNone                MutationErrorKind = ""
	MutationErrorUnsupportedProvider MutationErrorKind = "unsupported_provider"
)

type MemoryAction string

const (
	MemoryActionNone            MemoryAction = ""
	MemoryActionRefresh         MemoryAction = "refresh"
	MemoryActionClearPersistent MemoryAction = "clear_persistent"
	MemoryActionClearSession    MemoryAction = "clear_session"
)

type MutationFeedback struct {
	AssistantMessage   string
	StatusMessage      string
	Snapshot           UISnapshot
	APIKeyReady        bool
	ValidationErr      error
	ErrorKind          MutationErrorKind
	SupportedProviders []string
}

type MemoryFeedback struct {
	AssistantMessage string
	StatusMessage    string
	Stats            *MemoryStats
	Action           MemoryAction
}

func UpdateAPIKeySetting(ctx context.Context, client ChatClient, configPath, envName string) (*MutationFeedback, error) {
	result, err := UpdateAPIKeyEnvVar(ctx, configPath, envName)
	if err != nil {
		return nil, err
	}
	return BuildUpdateAPIKeyFeedback(client, result), nil
}

func BuildUpdateAPIKeyFeedback(client ChatClient, result *ConfigMutationResult) *MutationFeedback {
	feedback := &MutationFeedback{
		Snapshot:      ReadUISnapshot(context.Background(), client),
		APIKeyReady:   result.APIKeyReady,
		ValidationErr: result.ValidationErr,
	}
	if result.ValidationErr == nil {
		feedback.AssistantMessage = fmt.Sprintf("已切换 API Key 环境变量为 %s，并验证通过。", result.APIKeyEnvVar)
		feedback.StatusMessage = "已更新 API Key 配置"
		return feedback
	}
	if errors.Is(result.ValidationErr, ErrAPIKeyMissing) {
		feedback.AssistantMessage = fmt.Sprintf("环境变量 %s 未设置。请使用 /apikey <env_name> 切换，或使用 /exit 退出。", result.APIKeyEnvVar)
		return feedback
	}
	if errors.Is(result.ValidationErr, ErrInvalidAPIKey) {
		feedback.AssistantMessage = fmt.Sprintf("环境变量 %s 中的 API Key 无效：%v。请使用 /apikey <env_name>、/provider <name> 或 /switch <model> 调整配置。", result.APIKeyEnvVar, result.ValidationErr)
		return feedback
	}
	feedback.AssistantMessage = fmt.Sprintf("环境变量 %s 中的 API Key 校验失败：%v。请使用 /apikey <env_name>、/provider <name> 或 /switch <model> 调整配置。", result.APIKeyEnvVar, result.ValidationErr)
	return feedback
}

func SwitchProviderSetting(ctx context.Context, client ChatClient, configPath, providerName string) (*MutationFeedback, error) {
	if _, ok := NormalizeProviderName(providerName); !ok {
		return &MutationFeedback{
			ErrorKind:          MutationErrorUnsupportedProvider,
			AssistantMessage:   fmt.Sprintf("Unsupported provider: %s", strings.TrimSpace(providerName)),
			SupportedProviders: SupportedProviders(),
			Snapshot:           ReadUISnapshot(context.Background(), client),
		}, nil
	}

	result, err := SwitchProvider(ctx, configPath, providerName)
	if err != nil {
		return nil, err
	}
	return BuildSwitchProviderFeedback(client, result), nil
}

func BuildSwitchProviderFeedback(client ChatClient, result *ConfigMutationResult) *MutationFeedback {
	feedback := &MutationFeedback{
		Snapshot:      ReadUISnapshot(context.Background(), client),
		APIKeyReady:   result.APIKeyReady,
		ValidationErr: result.ValidationErr,
	}
	if strings.TrimSpace(result.Provider) != "" {
		feedback.Snapshot.ProviderName = strings.TrimSpace(result.Provider)
	}
	if strings.TrimSpace(result.Model) != "" {
		feedback.Snapshot.CurrentModel = strings.TrimSpace(result.Model)
	}
	if result.ValidationErr == nil {
		feedback.AssistantMessage = fmt.Sprintf("已切换 provider 为 %s，当前模型已重置为默认值：%s。", result.Provider, result.Model)
		feedback.StatusMessage = "provider 已切换"
		return feedback
	}
	if errors.Is(result.ValidationErr, ErrAPIKeyMissing) {
		feedback.AssistantMessage = fmt.Sprintf("已切换 provider 为 %s，但环境变量 %s 未设置。请使用 /apikey <env_name> 或设置该环境变量。", result.Provider, result.APIKeyEnvVar)
		return feedback
	}
	feedback.AssistantMessage = fmt.Sprintf("已切换 provider 为 %s，但 API Key 校验失败：%v。你仍可继续使用 /apikey <env_name>、/provider <name> 或 /switch <model> 调整配置。", result.Provider, result.ValidationErr)
	return feedback
}

func SwitchModelSetting(ctx context.Context, client ChatClient, configPath, model string) (*MutationFeedback, error) {
	result, err := SwitchModel(ctx, configPath, model)
	if err != nil {
		return nil, err
	}
	return BuildSwitchModelFeedback(client, result), nil
}

func BuildSwitchModelFeedback(client ChatClient, result *ConfigMutationResult) *MutationFeedback {
	feedback := &MutationFeedback{
		Snapshot:      ReadUISnapshot(context.Background(), client),
		APIKeyReady:   result.APIKeyReady,
		ValidationErr: result.ValidationErr,
	}
	if strings.TrimSpace(result.Model) != "" {
		feedback.Snapshot.CurrentModel = strings.TrimSpace(result.Model)
	}
	if result.ValidationErr == nil {
		feedback.AssistantMessage = fmt.Sprintf("已切换模型为：%s", result.Model)
		feedback.StatusMessage = "模型已切换"
		return feedback
	}
	if errors.Is(result.ValidationErr, ErrAPIKeyMissing) {
		feedback.AssistantMessage = fmt.Sprintf("已切换模型为 %s，但环境变量 %s 未设置。", result.Model, result.APIKeyEnvVar)
		return feedback
	}
	feedback.AssistantMessage = fmt.Sprintf("已切换模型为 %s，但 API Key 校验失败：%v。", result.Model, result.ValidationErr)
	return feedback
}

func LoadMemoryStatsFeedback(ctx context.Context, client ChatClient) (*MemoryFeedback, error) {
	stats, err := client.GetMemoryStats(ctx)
	if err != nil {
		return nil, err
	}
	if stats == nil {
		stats = &MemoryStats{}
	}
	return &MemoryFeedback{
		Stats:         stats,
		Action:        MemoryActionRefresh,
		StatusMessage: "Memory 已刷新",
		AssistantMessage: fmt.Sprintf(
			"Memory stats:\n  Persistent: %d\n  Session: %d\n  Total: %d\n  TopK: %d\n  Min score: %.2f\n  File: %s\n  Types: %s",
			stats.PersistentItems, stats.SessionItems, stats.TotalItems, stats.TopK, stats.MinScore, stats.Path, FormatMemoryTypeStats(stats.ByType),
		),
	}, nil
}

func ClearPersistentMemoryFeedback(ctx context.Context, client ChatClient) (*MemoryFeedback, error) {
	if err := client.ClearMemory(ctx); err != nil {
		return nil, err
	}
	stats, _ := client.GetMemoryStats(ctx)
	if stats == nil {
		stats = &MemoryStats{}
	}
	return &MemoryFeedback{
		Stats:            stats,
		Action:           MemoryActionClearPersistent,
		StatusMessage:    "已清空持久记忆",
		AssistantMessage: "已清空本地持久记忆",
	}, nil
}

func ClearSessionContextFeedback(ctx context.Context, client ChatClient) (*MemoryFeedback, error) {
	if err := client.ClearSessionMemory(ctx); err != nil {
		return nil, err
	}
	stats, _ := client.GetMemoryStats(ctx)
	if stats == nil {
		stats = &MemoryStats{}
	}
	return &MemoryFeedback{
		Stats:            stats,
		Action:           MemoryActionClearSession,
		StatusMessage:    "会话上下文已清空",
		AssistantMessage: "已清空当前会话上下文",
	}, nil
}

func FormatMemoryTypeStats(byType map[string]int) string {
	if len(byType) == 0 {
		return "none"
	}

	ordered := []string{
		TypeUserPreference,
		TypeProjectRule,
		TypeCodeFact,
		TypeFixRecipe,
		TypeSessionMemory,
	}

	parts := make([]string, 0, len(byType))
	for _, key := range ordered {
		if count := byType[key]; count > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", key, count))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}
