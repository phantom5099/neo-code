package runtime

import (
	"context"
	"errors"
	"strings"

	agentcontext "neo-code/internal/context"
	contextcompact "neo-code/internal/context/compact"
	"neo-code/internal/provider"
	"neo-code/internal/provider/streaming"
	providertypes "neo-code/internal/provider/types"
)

type compactSummaryGenerator struct {
	providerFactory ProviderFactory
	providerConfig  provider.RuntimeConfig
	model           string
}

func newCompactSummaryGenerator(
	providerFactory ProviderFactory,
	providerCfg provider.RuntimeConfig,
	model string,
) contextcompact.SummaryGenerator {
	return &compactSummaryGenerator{
		providerFactory: providerFactory,
		providerConfig:  providerCfg,
		model:           strings.TrimSpace(model),
	}
}

// Generate 使用冻结后的 provider 配置为 compact 生成语义摘要。
func (g *compactSummaryGenerator) Generate(ctx context.Context, input contextcompact.SummaryInput) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if g.providerFactory == nil {
		return "", errors.New("runtime: compact summary generator provider factory is nil")
	}
	if strings.TrimSpace(g.providerConfig.Driver) == "" ||
		strings.TrimSpace(g.providerConfig.BaseURL) == "" ||
		strings.TrimSpace(g.providerConfig.APIKey) == "" {
		return "", errors.New("runtime: compact summary generator provider config is incomplete")
	}

	prompt := agentcontext.BuildCompactPrompt(agentcontext.CompactPromptInput{
		Mode:                     string(input.Mode),
		ManualStrategy:           input.Config.ManualStrategy,
		ManualKeepRecentMessages: input.Config.ManualKeepRecentMessages,
		ArchivedMessageCount:     input.ArchivedMessageCount,
		MaxSummaryChars:          input.Config.MaxSummaryChars,
		ArchivedMessages:         input.ArchivedMessages,
		RetainedMessages:         input.RetainedMessages,
	})

	modelProvider, err := g.providerFactory.Build(ctx, g.providerConfig)
	if err != nil {
		return "", err
	}

	outcome := generateStreamingMessage(ctx, modelProvider, providertypes.GenerateRequest{
		Model:        g.model,
		SystemPrompt: prompt.SystemPrompt,
		Messages: []providertypes.Message{{
			Role:    providertypes.RoleUser,
			Content: prompt.UserPrompt,
		}},
	}, streaming.Hooks{})
	if outcome.err != nil {
		return "", outcome.err
	}

	message := outcome.message
	if len(message.ToolCalls) > 0 {
		return "", errors.New("runtime: compact summary response must not contain tool calls")
	}

	summary := strings.TrimSpace(message.Content)
	if summary == "" {
		return "", errors.New("runtime: compact summary response is empty")
	}
	return summary, nil
}
