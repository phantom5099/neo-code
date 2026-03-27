package services

import (
	"context"

	"neo-code/internal/agentruntime/interaction"
)

type MutationErrorKind = interaction.MutationErrorKind

const (
	MutationErrorNone                = interaction.MutationErrorNone
	MutationErrorUnsupportedProvider = interaction.MutationErrorUnsupportedProvider
)

type MemoryAction = interaction.MemoryAction

const (
	MemoryActionNone            = interaction.MemoryActionNone
	MemoryActionRefresh         = interaction.MemoryActionRefresh
	MemoryActionClearPersistent = interaction.MemoryActionClearPersistent
	MemoryActionClearSession    = interaction.MemoryActionClearSession
)

type MutationFeedback = interaction.MutationFeedback
type MemoryFeedback = interaction.MemoryFeedback

func UpdateAPIKeySetting(ctx context.Context, client ChatClient, configPath, envName string) (*MutationFeedback, error) {
	return interaction.UpdateAPIKeySetting(ctx, client, configPath, envName)
}

func BuildUpdateAPIKeyFeedback(client ChatClient, result *ConfigMutationResult) *MutationFeedback {
	return interaction.BuildUpdateAPIKeyFeedback(client, result)
}

func SwitchProviderSetting(ctx context.Context, client ChatClient, configPath, providerName string) (*MutationFeedback, error) {
	return interaction.SwitchProviderSetting(ctx, client, configPath, providerName)
}

func BuildSwitchProviderFeedback(client ChatClient, result *ConfigMutationResult) *MutationFeedback {
	return interaction.BuildSwitchProviderFeedback(client, result)
}

func SwitchModelSetting(ctx context.Context, client ChatClient, configPath, model string) (*MutationFeedback, error) {
	return interaction.SwitchModelSetting(ctx, client, configPath, model)
}

func BuildSwitchModelFeedback(client ChatClient, result *ConfigMutationResult) *MutationFeedback {
	return interaction.BuildSwitchModelFeedback(client, result)
}

func LoadMemoryStatsFeedback(ctx context.Context, client ChatClient) (*MemoryFeedback, error) {
	return interaction.LoadMemoryStatsFeedback(ctx, client)
}

func ClearPersistentMemoryFeedback(ctx context.Context, client ChatClient) (*MemoryFeedback, error) {
	return interaction.ClearPersistentMemoryFeedback(ctx, client)
}

func ClearSessionContextFeedback(ctx context.Context, client ChatClient) (*MemoryFeedback, error) {
	return interaction.ClearSessionContextFeedback(ctx, client)
}

func FormatMemoryTypeStats(byType map[string]int) string {
	return interaction.FormatMemoryTypeStats(byType)
}
