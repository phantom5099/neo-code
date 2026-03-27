package interaction

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type SessionService interface {
	Bootstrap(ctx context.Context) BootstrapData
	HandleInput(ctx context.Context, snapshot SessionSnapshot, input string) (InputResult, error)
	ContinueAfterStream(ctx context.Context, snapshot SessionSnapshot) (TurnResolution, error)
	RefreshMemory(ctx context.Context) (*MemoryFeedback, error)
}

type SessionSnapshot struct {
	Messages        []SessionMessage
	ActiveModel     string
	APIKeyReady     bool
	Generating      bool
	ApprovalRunning bool
	PendingApproval *ToolApprovalRequest
	WorkspaceRoot   string
}

type InputResult struct {
	Messages         []SessionMessage
	TurnResolution   *TurnResolution
	MutationFeedback *MutationFeedback
	MemoryFeedback   *MemoryFeedback
	Stream           <-chan string
	StatusMessage    string
	HistoryEntry     string
	OpenHelp         bool
	Quit             bool
	ReportError      error
}

type runtimeSessionService struct {
	controller Controller
}

func NewSessionService(controller Controller) SessionService {
	return &runtimeSessionService{controller: controller}
}

func (s *runtimeSessionService) Bootstrap(ctx context.Context) BootstrapData {
	if s.controller == nil {
		return BootstrapData{}
	}
	return s.controller.Bootstrap(ctx)
}

func (s *runtimeSessionService) RefreshMemory(ctx context.Context) (*MemoryFeedback, error) {
	if s.controller == nil {
		return nil, errors.New("session service has no controller")
	}
	return s.controller.LoadMemoryStats(ctx)
}

func (s *runtimeSessionService) ContinueAfterStream(ctx context.Context, snapshot SessionSnapshot) (TurnResolution, error) {
	if s.controller == nil {
		return TurnResolution{}, errors.New("session service has no controller")
	}
	return s.controller.ResolveAssistantTurn(ctx, conversationRequest(snapshot), lastAssistantContent(snapshot.Messages))
}

func (s *runtimeSessionService) HandleInput(ctx context.Context, snapshot SessionSnapshot, input string) (InputResult, error) {
	if s.controller == nil {
		return InputResult{}, errors.New("session service has no controller")
	}

	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return InputResult{}, nil
	}
	if strings.HasPrefix(trimmed, "/") {
		return s.handleCommand(ctx, snapshot, trimmed)
	}
	return s.handleConversationInput(ctx, snapshot, trimmed)
}

func (s *runtimeSessionService) handleConversationInput(ctx context.Context, snapshot SessionSnapshot, input string) (InputResult, error) {
	if snapshot.Generating {
		return InputResult{StatusMessage: "Generating..."}, nil
	}
	if !snapshot.APIKeyReady {
		return InputResult{
			Messages:    []SessionMessage{assistantNotice("当前 API Key 不可用，请使用 /apikey、/provider 或 /switch 调整配置。")},
			ReportError: errors.New("api key unavailable"),
		}, nil
	}
	if snapshot.PendingApproval != nil {
		return InputResult{
			Messages: []SessionMessage{assistantNotice("当前有待确认的安全审批，请先使用 /y 或 /n 处理。")},
		}, nil
	}

	req := conversationRequest(snapshot)
	req.Messages = append(req.Messages, SessionMessage{Role: "user", Content: input, Kind: MessageKindPlain})
	stream, err := s.controller.StartChat(ctx, req)
	if err != nil {
		return InputResult{}, err
	}

	return InputResult{
		Messages: []SessionMessage{
			{Role: "user", Content: input, Kind: MessageKindPlain},
			{Role: "assistant", Content: "", Kind: MessageKindPlain},
		},
		Stream:       stream,
		HistoryEntry: input,
	}, nil
}

func (s *runtimeSessionService) handleCommand(ctx context.Context, snapshot SessionSnapshot, input string) (InputResult, error) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return InputResult{}, nil
	}

	cmd := fields[0]
	args := fields[1:]
	if !snapshot.APIKeyReady && !isRecoveryCommand(cmd) {
		return InputResult{
			Messages: []SessionMessage{assistantNotice("当前 API Key 不可用。现在仅可使用 /apikey、/provider、/switch、/help、/pwd(/workspace) 和 /exit。")},
		}, nil
	}

	switch cmd {
	case "/help":
		return InputResult{OpenHelp: true}, nil
	case "/exit", "/quit", "/q":
		return InputResult{Quit: true}, nil
	case "/apikey":
		if len(args) == 0 {
			return InputResult{Messages: []SessionMessage{assistantNotice("用法：/apikey <env_name>")}}, nil
		}
		feedback, err := s.controller.UpdateAPIKey(ctx, args[0])
		return InputResult{MutationFeedback: feedback}, err
	case "/provider":
		if len(args) == 0 {
			return InputResult{Messages: []SessionMessage{assistantNotice(
				fmt.Sprintf("用法：/provider <name>\n支持的 provider：\n  - %s", strings.Join(SupportedProviders(), "\n  - ")),
			)}}, nil
		}
		feedback, err := s.controller.SwitchProvider(ctx, strings.Join(args, " "))
		return InputResult{MutationFeedback: feedback}, err
	case "/switch":
		if len(args) == 0 {
			return InputResult{Messages: []SessionMessage{assistantNotice("用法：/switch <model>")}}, nil
		}
		feedback, err := s.controller.SwitchModel(ctx, strings.Join(args, " "))
		return InputResult{MutationFeedback: feedback}, err
	case "/pwd", "/workspace":
		if len(args) > 0 {
			return InputResult{Messages: []SessionMessage{assistantNotice("用法：/pwd 或 /workspace")}}, nil
		}
		root := strings.TrimSpace(snapshot.WorkspaceRoot)
		if root == "" {
			return InputResult{Messages: []SessionMessage{assistantNotice("当前工作区：unknown")}}, nil
		}
		return InputResult{Messages: []SessionMessage{assistantNotice(fmt.Sprintf("当前工作区：%s", root))}}, nil
	case "/memory":
		feedback, err := s.controller.LoadMemoryStats(ctx)
		return InputResult{MemoryFeedback: feedback}, err
	case "/clear-memory":
		if len(args) == 0 || args[0] != "confirm" {
			return InputResult{Messages: []SessionMessage{assistantNotice("该命令会清空持久记忆，请使用 /clear-memory confirm。")}}, nil
		}
		feedback, err := s.controller.ClearPersistentMemory(ctx)
		return InputResult{MemoryFeedback: feedback}, err
	case "/clear-context":
		feedback, err := s.controller.ClearSessionContext(ctx)
		return InputResult{MemoryFeedback: feedback}, err
	case "/y", "/n":
		return s.handleApprovalCommand(ctx, snapshot, cmd, args)
	default:
		return InputResult{
			Messages: []SessionMessage{assistantNotice(fmt.Sprintf("未知命令：%s。输入 /help 查看可用命令。", cmd))},
		}, nil
	}
}

func (s *runtimeSessionService) handleApprovalCommand(ctx context.Context, snapshot SessionSnapshot, cmd string, args []string) (InputResult, error) {
	if len(args) > 0 {
		return InputResult{Messages: []SessionMessage{assistantNotice(fmt.Sprintf("用法：%s", cmd))}}, nil
	}
	if snapshot.PendingApproval == nil {
		return InputResult{Messages: []SessionMessage{assistantNotice("当前没有待确认的安全审批。")}}, nil
	}
	if cmd == "/y" && snapshot.ApprovalRunning {
		return InputResult{Messages: []SessionMessage{assistantNotice("另一个工具仍在运行，请稍后再试 /y。")}}, nil
	}

	decision := ApprovalDecisionReject
	if cmd == "/y" {
		decision = ApprovalDecisionApprove
	}
	resolution, err := s.controller.ResolveApproval(ctx, conversationRequest(snapshot), *snapshot.PendingApproval, decision)
	if err != nil {
		return InputResult{}, err
	}
	return InputResult{TurnResolution: &resolution}, nil
}

func conversationRequest(snapshot SessionSnapshot) ConversationRequest {
	return ConversationRequest{
		Messages:    snapshot.Messages,
		ActiveModel: snapshot.ActiveModel,
	}
}

func lastAssistantContent(messages []SessionMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			return messages[i].Content
		}
	}
	return ""
}

func assistantNotice(content string) SessionMessage {
	return SessionMessage{
		Role:      "assistant",
		Content:   content,
		Kind:      MessageKindPlain,
		Transient: true,
	}
}

func isRecoveryCommand(cmd string) bool {
	switch cmd {
	case "/apikey", "/provider", "/help", "/switch", "/pwd", "/workspace", "/y", "/n", "/exit", "/quit", "/q":
		return true
	default:
		return false
	}
}
