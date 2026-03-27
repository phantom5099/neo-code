package services

import (
	"context"
	"fmt"
	"strings"
)

type ApprovalDecision string

const (
	ApprovalDecisionApprove ApprovalDecision = "approve"
	ApprovalDecisionReject  ApprovalDecision = "reject"
)

type Controller interface {
	Bootstrap(ctx context.Context) BootstrapData
	StartChat(ctx context.Context, req ConversationRequest) (<-chan string, error)
	ResolveAssistantTurn(ctx context.Context, req ConversationRequest, assistantContent string) (TurnResolution, error)
	ResolveApproval(ctx context.Context, req ConversationRequest, pending ToolApprovalRequest, decision ApprovalDecision) (TurnResolution, error)
	UpdateAPIKey(ctx context.Context, envName string) (*MutationFeedback, error)
	SwitchProvider(ctx context.Context, providerName string) (*MutationFeedback, error)
	SwitchModel(ctx context.Context, model string) (*MutationFeedback, error)
	LoadMemoryStats(ctx context.Context) (*MemoryFeedback, error)
	ClearPersistentMemory(ctx context.Context) (*MemoryFeedback, error)
	ClearSessionContext(ctx context.Context) (*MemoryFeedback, error)
}

type BootstrapData struct {
	Snapshot      UISnapshot
	MemoryStats   MemoryStats
	ResumeSummary SessionMessage
	APIKeyReady   bool
}

type ConversationRequest struct {
	Messages    []SessionMessage
	ActiveModel string
}

type TurnResolution struct {
	Messages        []SessionMessage
	PendingApproval *ToolApprovalRequest
	StatusMessage   string
	TouchedPaths    []string
	Stream          <-chan string
}

type runtimeController struct {
	client     ChatClient
	configPath string
}

func NewRuntimeController(client ChatClient, configPath string) Controller {
	return &runtimeController{
		client:     client,
		configPath: strings.TrimSpace(configPath),
	}
}

func (c *runtimeController) Bootstrap(ctx context.Context) BootstrapData {
	data := BootstrapData{
		Snapshot:    ReadUISnapshot(ctx, c.client),
		APIKeyReady: RuntimeAPIKeyReady(),
	}
	if c.client != nil {
		if stats, err := c.client.GetMemoryStats(ctx); err == nil && stats != nil {
			data.MemoryStats = *stats
		}
	}
	if summaryProvider, ok := c.client.(WorkingSessionSummaryProvider); ok && summaryProvider != nil {
		if summary, err := summaryProvider.GetWorkingSessionSummary(ctx); err == nil {
			data.ResumeSummary = ResumeSummaryMessage(summary)
		}
	}
	return data
}

func (c *runtimeController) StartChat(ctx context.Context, req ConversationRequest) (<-chan string, error) {
	if c.client == nil {
		return nil, fmt.Errorf("runtime controller has no chat client")
	}
	return c.client.Chat(ctx, BuildRequestMessages(req.Messages), strings.TrimSpace(req.ActiveModel))
}

func (c *runtimeController) ResolveAssistantTurn(ctx context.Context, req ConversationRequest, assistantContent string) (TurnResolution, error) {
	resolution := TurnResolution{StatusMessage: "生成完成"}
	plan := FirstToolExecutionPlan(assistantContent)
	if plan == nil {
		return resolution, nil
	}

	resolution.Messages = append(resolution.Messages, plan.StatusContext)
	resolution.Messages[len(resolution.Messages)-1].Transient = true
	resolution.TouchedPaths = append(resolution.TouchedPaths, plan.TouchedPaths...)

	result := ExecuteToolCall(plan.Call)
	if result == nil {
		return TurnResolution{}, fmt.Errorf("tool execution failed: empty result")
	}

	completion := PlanToolResult(plan.Call, result)
	resolution.StatusMessage = completion.StatusMessage
	resolution.TouchedPaths = append(resolution.TouchedPaths, completion.TouchedPaths...)
	if completion.PendingApproval != nil {
		resolution.PendingApproval = completion.PendingApproval
		resolution.Messages = append(resolution.Messages, SessionMessage{
			Role:      "assistant",
			Content:   completion.PendingApproval.AssistantMessage,
			Kind:      MessageKindPlain,
			Transient: true,
		})
		return resolution, nil
	}

	if strings.TrimSpace(completion.SystemContextMessage.Content) != "" {
		resolution.Messages = append(resolution.Messages, completion.SystemContextMessage)
	}
	if completion.ContinueConversation {
		stream, err := c.StartChat(ctx, ConversationRequest{
			Messages:    append(append([]SessionMessage{}, req.Messages...), resolution.Messages...),
			ActiveModel: req.ActiveModel,
		})
		if err != nil {
			return TurnResolution{}, err
		}
		resolution.Stream = stream
	}
	return resolution, nil
}

func (c *runtimeController) ResolveApproval(ctx context.Context, req ConversationRequest, pending ToolApprovalRequest, decision ApprovalDecision) (TurnResolution, error) {
	if decision == ApprovalDecisionReject {
		toolName := strings.TrimSpace(pending.Call.Tool)
		if toolName == "" {
			toolName = "unknown"
		}
		return TurnResolution{
			Messages: []SessionMessage{{
				Role:      "assistant",
				Content:   fmt.Sprintf("已拒绝工具 %s，本次不会执行。", toolName),
				Kind:      MessageKindPlain,
				Transient: true,
			}},
			StatusMessage: "已拒绝审批",
		}, nil
	}

	ApproveToolCall(pending.ToolType, pending.Target)
	plan := PlanToolExecution(pending.Call)
	resolution := TurnResolution{
		Messages: []SessionMessage{
			{
				Role:      "assistant",
				Content:   fmt.Sprintf("已批准，开始执行工具 %s。", strings.TrimSpace(pending.Call.Tool)),
				Kind:      MessageKindPlain,
				Transient: true,
			},
			{
				Role:      plan.StatusContext.Role,
				Content:   plan.StatusContext.Content,
				Kind:      plan.StatusContext.Kind,
				Transient: true,
			},
		},
		StatusMessage: "Running tool...",
		TouchedPaths:  append([]string{}, plan.TouchedPaths...),
	}

	result := ExecuteToolCall(plan.Call)
	if result == nil {
		return TurnResolution{}, fmt.Errorf("tool execution failed: empty result")
	}

	completion := PlanToolResult(plan.Call, result)
	resolution.StatusMessage = completion.StatusMessage
	resolution.TouchedPaths = append(resolution.TouchedPaths, completion.TouchedPaths...)
	if completion.PendingApproval != nil {
		resolution.PendingApproval = completion.PendingApproval
		resolution.Messages = append(resolution.Messages, SessionMessage{
			Role:      "assistant",
			Content:   completion.PendingApproval.AssistantMessage,
			Kind:      MessageKindPlain,
			Transient: true,
		})
		return resolution, nil
	}

	if strings.TrimSpace(completion.SystemContextMessage.Content) != "" {
		resolution.Messages = append(resolution.Messages, completion.SystemContextMessage)
	}
	if completion.ContinueConversation {
		stream, err := c.StartChat(ctx, ConversationRequest{
			Messages:    append(append([]SessionMessage{}, req.Messages...), resolution.Messages...),
			ActiveModel: req.ActiveModel,
		})
		if err != nil {
			return TurnResolution{}, err
		}
		resolution.Stream = stream
	}
	return resolution, nil
}

func (c *runtimeController) UpdateAPIKey(ctx context.Context, envName string) (*MutationFeedback, error) {
	return UpdateAPIKeySetting(ctx, c.client, c.configPath, envName)
}

func (c *runtimeController) SwitchProvider(ctx context.Context, providerName string) (*MutationFeedback, error) {
	return SwitchProviderSetting(ctx, c.client, c.configPath, providerName)
}

func (c *runtimeController) SwitchModel(ctx context.Context, model string) (*MutationFeedback, error) {
	return SwitchModelSetting(ctx, c.client, c.configPath, model)
}

func (c *runtimeController) LoadMemoryStats(ctx context.Context) (*MemoryFeedback, error) {
	return LoadMemoryStatsFeedback(ctx, c.client)
}

func (c *runtimeController) ClearPersistentMemory(ctx context.Context) (*MemoryFeedback, error) {
	return ClearPersistentMemoryFeedback(ctx, c.client)
}

func (c *runtimeController) ClearSessionContext(ctx context.Context) (*MemoryFeedback, error) {
	return ClearSessionContextFeedback(ctx, c.client)
}
