package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"neo-code/internal/config"
	"neo-code/internal/tui/services"
	"neo-code/internal/tui/state"

	tea "github.com/charmbracelet/bubbletea"
)

type fakeChatClient struct {
	chatChunks       []string
	chatErr          error
	lastMessages     []services.Message
	lastModel        string
	memoryStats      *services.MemoryStats
	nilMemoryStats   bool
	memoryErr        error
	clearMemoryErr   error
	clearSessionErr  error
	defaultModelName string
}

func (f *fakeChatClient) Chat(_ context.Context, messages []services.Message, model string) (<-chan string, error) {
	f.lastMessages = append([]services.Message(nil), messages...)
	f.lastModel = model
	if f.chatErr != nil {
		return nil, f.chatErr
	}

	ch := make(chan string, len(f.chatChunks))
	for _, chunk := range f.chatChunks {
		ch <- chunk
	}
	close(ch)
	return ch, nil
}

func (f *fakeChatClient) GetMemoryStats(context.Context) (*services.MemoryStats, error) {
	if f.memoryErr != nil {
		return nil, f.memoryErr
	}
	if f.nilMemoryStats {
		return nil, nil
	}
	if f.memoryStats != nil {
		statsCopy := *f.memoryStats
		return &statsCopy, nil
	}
	return &services.MemoryStats{}, nil
}

func (f *fakeChatClient) ClearMemory(context.Context) error {
	return f.clearMemoryErr
}

func (f *fakeChatClient) ClearSessionMemory(context.Context) error {
	return f.clearSessionErr
}

func (f *fakeChatClient) DefaultModel() string {
	if f.defaultModelName != "" {
		return f.defaultModelName
	}
	return "test-model"
}

type fakeSessionService struct {
	bootstrapFn           func(context.Context) services.BootstrapData
	handleInputFn         func(context.Context, services.SessionSnapshot, string) (services.InputResult, error)
	continueAfterStreamFn func(context.Context, services.SessionSnapshot) (services.TurnResolution, error)
	refreshMemoryFn       func(context.Context) (*services.MemoryFeedback, error)
}

func (f *fakeSessionService) Bootstrap(ctx context.Context) services.BootstrapData {
	if f != nil && f.bootstrapFn != nil {
		return f.bootstrapFn(ctx)
	}
	return services.BootstrapData{}
}

func (f *fakeSessionService) HandleInput(ctx context.Context, snapshot services.SessionSnapshot, input string) (services.InputResult, error) {
	if f != nil && f.handleInputFn != nil {
		return f.handleInputFn(ctx, snapshot, input)
	}
	return services.InputResult{}, nil
}

func (f *fakeSessionService) ContinueAfterStream(ctx context.Context, snapshot services.SessionSnapshot) (services.TurnResolution, error) {
	if f != nil && f.continueAfterStreamFn != nil {
		return f.continueAfterStreamFn(ctx, snapshot)
	}
	return services.TurnResolution{}, nil
}

func (f *fakeSessionService) RefreshMemory(ctx context.Context) (*services.MemoryFeedback, error) {
	if f != nil && f.refreshMemoryFn != nil {
		return f.refreshMemoryFn(ctx)
	}
	return &services.MemoryFeedback{}, nil
}

func newRuntimeSessionService(client *fakeChatClient) services.SessionService {
	return services.NewSessionService(services.NewRuntimeController(client, "config.yaml"))
}

func newTestModel(t *testing.T, client *fakeChatClient) *Model {
	t.Helper()
	cfg := config.DefaultAppConfig()
	origGlobalConfig := config.GlobalAppConfig
	t.Cleanup(func() {
		config.GlobalAppConfig = origGlobalConfig
	})
	config.GlobalAppConfig = cfg

	m := NewModel(newRuntimeSessionService(client), "")
	m.ui.Width = 80
	m.ui.Height = 24
	m.applyBootstrap(m.session.Bootstrap(context.Background()))
	m.syncLayout()
	return &m
}

func lastMessageContent(t *testing.T, m Model) string {
	t.Helper()
	if len(m.chat.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	return m.chat.Messages[len(m.chat.Messages)-1].Content
}

func assertLastMessageContains(t *testing.T, m Model, want string) {
	t.Helper()
	if !strings.Contains(lastMessageContent(t, m), want) {
		t.Fatalf("expected last message to contain %q, got %q", want, lastMessageContent(t, m))
	}
}

func runCommand(t *testing.T, m *Model, input string) Model {
	t.Helper()

	updated, cmd := m.handleCommand(input)
	got := updated.(Model)
	if cmd != nil {
		updated, _ = got.Update(cmd())
		got = updated.(Model)
	}
	return got
}

func TestHandleSubmitEmptyInputNoOp(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.textarea.SetValue("   ")

	updated, cmd := m.handleSubmit()
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no command for empty input")
	}
	if len(got.chat.Messages) != 0 {
		t.Fatalf("expected no messages, got %d", len(got.chat.Messages))
	}
}

func TestHandleSubmitFromHelpModeReturnsToChat(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.ui.Mode = state.ModeHelp
	m.textarea.SetValue("help")

	updated, cmd := m.handleSubmit()
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no command when leaving help mode")
	}
	if got.ui.Mode != state.ModeChat {
		t.Fatalf("expected chat mode, got %v", got.ui.Mode)
	}
	if len(got.chat.Messages) != 0 {
		t.Fatalf("expected no messages while leaving help mode, got %d", len(got.chat.Messages))
	}
}

func TestHandleSubmitRequiresReadyAPIKey(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = false
	m.textarea.SetValue("hello")

	updated, cmd := m.handleSubmit()
	got := updated.(Model)

	if cmd == nil {
		t.Fatal("expected command when API key is not ready")
	}
	updated, _ = got.Update(cmd())
	got = updated.(Model)
	if len(got.chat.Messages) != 1 {
		t.Fatalf("expected one assistant warning, got %d messages", len(got.chat.Messages))
	}
	if got.chat.Messages[0].Role != "assistant" {
		t.Fatalf("expected assistant warning, got %+v", got.chat.Messages[0])
	}
	if !strings.Contains(got.chat.Messages[0].Content, "API Key") {
		t.Fatalf("expected API key warning, got %q", got.chat.Messages[0].Content)
	}
}

func TestHandleSubmitStartsStreamingConversation(t *testing.T) {
	client := &fakeChatClient{chatChunks: []string{"hello back"}}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true
	m.textarea.SetValue("hello")

	updated, cmd := m.handleSubmit()
	got := updated.(Model)

	if cmd == nil {
		t.Fatal("expected streaming command")
	}
	if got.chat.Generating {
		t.Fatal("expected submit to stay non-generating until input result is applied")
	}

	msg := cmd()
	if _, ok := msg.(InputHandledMsg); !ok {
		t.Fatalf("expected InputHandledMsg, got %T", msg)
	}
	updated, cmd = got.Update(msg)
	got = updated.(Model)
	if !got.chat.Generating {
		t.Fatal("expected generating=true after input result")
	}
	if len(got.chat.Messages) != 2 {
		t.Fatalf("expected user and assistant placeholder, got %d messages", len(got.chat.Messages))
	}
	if got.chat.Messages[0].Role != "user" || got.chat.Messages[0].Content != "hello" {
		t.Fatalf("unexpected user message: %+v", got.chat.Messages[0])
	}
	if got.chat.Messages[1].Role != "assistant" || got.chat.Messages[1].Content != "" {
		t.Fatalf("expected assistant placeholder, got %+v", got.chat.Messages[1])
	}
	if len(got.ui.CommandHistory) != 1 || got.ui.CommandHistory[0] != "hello" {
		t.Fatalf("expected command history to record input, got %+v", got.ui.CommandHistory)
	}
	if cmd == nil {
		t.Fatal("expected follow-up stream command")
	}
	msg = cmd()
	chunk, ok := msg.(StreamChunkMsg)
	if !ok {
		t.Fatalf("expected StreamChunkMsg after input result, got %T", msg)
	}
	if chunk.Content != "hello back" {
		t.Fatalf("expected first stream chunk, got %q", chunk.Content)
	}
	if len(client.lastMessages) != 1 || client.lastMessages[0].Role != "user" || client.lastMessages[0].Content != "hello" {
		t.Fatalf("expected streamed context to contain only the user message, got %+v", client.lastMessages)
	}
}

func TestHandleCommandRejectsNonRecoveryCommandWithoutAPIKey(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = false

	got := runCommand(t, m, "/memory")
	if len(got.chat.Messages) != 1 {
		t.Fatalf("expected one warning message, got %d", len(got.chat.Messages))
	}
	if !strings.Contains(got.chat.Messages[0].Content, "API Key") {
		t.Fatalf("expected API key guidance, got %q", got.chat.Messages[0].Content)
	}
}

func TestHandleCommandAPIKeyRequiresArgument(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)

	got := runCommand(t, m, "/apikey")
	assertLastMessageContains(t, got, "/apikey <env_name>")
}

func TestHandleCommandAPIKeyRequiresLoadedConfig(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, input string) (services.InputResult, error) {
			if input != "/apikey TEST_ENV" {
				t.Fatalf("unexpected input %q", input)
			}
			return services.InputResult{}, errors.New("app config is not loaded")
		},
	}
	got := runCommand(t, m, "/apikey TEST_ENV")
	assertLastMessageContains(t, got, "not loaded")
}

func TestHandleCommandAPIKeyEnvStillMissing(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{
				MutationFeedback: &services.MutationFeedback{
					AssistantMessage: "Environment variable MISSING_ENV is not set.",
					APIKeyReady:      false,
				},
			}, nil
		},
	}
	got := runCommand(t, m, "/apikey MISSING_ENV")

	if got.chat.APIKeyReady {
		t.Fatal("expected API key to remain not ready")
	}
	assertLastMessageContains(t, got, "MISSING_ENV")
}

func TestHandleCommandAPIKeyInvalidKey(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{
				MutationFeedback: &services.MutationFeedback{
					AssistantMessage: "The API key in environment variable BAD_ENV is invalid.",
					APIKeyReady:      false,
					ValidationErr:    services.ErrInvalidAPIKey,
				},
			}, nil
		},
	}
	got := runCommand(t, m, "/apikey BAD_ENV")

	if got.chat.APIKeyReady {
		t.Fatal("expected invalid key to mark API key as not ready")
	}
	assertLastMessageContains(t, got, "BAD_ENV")
}

func TestHandleCommandAPIKeyGenericValidationError(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{
				MutationFeedback: &services.MutationFeedback{
					AssistantMessage: "validation failed",
					APIKeyReady:      false,
					ValidationErr:    errors.New("validation failed"),
				},
			}, nil
		},
	}
	got := runCommand(t, m, "/apikey GENERIC_ENV")

	if got.chat.APIKeyReady {
		t.Fatal("expected generic validation failure to mark API key as not ready")
	}
	assertLastMessageContains(t, got, "validation failed")
}

func TestHandleCommandAPIKeySuccessWritesConfig(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, input string) (services.InputResult, error) {
			if input != "/apikey TEST_API_KEY_ENV" {
				t.Fatalf("unexpected input %q", input)
			}
			return services.InputResult{
				MutationFeedback: &services.MutationFeedback{
					AssistantMessage: "Switched the API key environment variable name to TEST_API_KEY_ENV and validated it successfully.",
					APIKeyReady:      true,
				},
			}, nil
		},
	}
	got := runCommand(t, m, "/apikey TEST_API_KEY_ENV")
	if !got.chat.APIKeyReady {
		t.Fatal("expected API key to be ready after validation")
	}
	if !strings.Contains(lastMessageContent(t, got), "TEST_API_KEY_ENV") {
		t.Fatalf("expected success message to mention env name, got %q", lastMessageContent(t, got))
	}
}

func TestHandleCommandAPIKeyWriteFailureRestoresPreviousEnvName(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{}, errors.New("write failed")
		},
	}
	got := runCommand(t, m, "/apikey NEW_ENV")

	if !got.chat.APIKeyReady {
		t.Fatal("expected previous API key readiness to be preserved")
	}
	if !strings.Contains(lastMessageContent(t, got), "write failed") {
		t.Fatalf("expected write failure message, got %q", lastMessageContent(t, got))
	}
}

func TestHandleCommandProviderWithoutRuntimeKeyMarksAPIKeyNotReady(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{
				MutationFeedback: &services.MutationFeedback{
					AssistantMessage: "Switched provider to openai, but environment variable MISSING_ENV is not set.",
					APIKeyReady:      false,
					Snapshot:         services.UISnapshot{ProviderName: "openai", CurrentModel: "gpt-5.4"},
					ValidationErr:    fmt.Errorf("%w: MISSING_ENV", services.ErrAPIKeyMissing),
				},
			}, nil
		},
	}
	got := runCommand(t, m, "/provider openai")

	if got.chat.APIKeyReady {
		t.Fatal("expected API key to become not ready when provider env var is missing")
	}
	if got.chat.ActiveModel == "" {
		t.Fatal("expected provider switch to reset active model")
	}
	if !strings.Contains(lastMessageContent(t, got), "openai") {
		t.Fatalf("expected provider switch message, got %q", lastMessageContent(t, got))
	}
}

func TestHandleCommandProviderRequiresArgument(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)

	got := runCommand(t, m, "/provider")
	assertLastMessageContains(t, got, "/provider <name>")
}

func TestHandleCommandProviderRejectsUnknownProvider(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{
				MutationFeedback: &services.MutationFeedback{
					ErrorKind:          services.MutationErrorUnsupportedProvider,
					AssistantMessage:   "Unsupported provider: unknown",
					SupportedProviders: []string{"openai", "anthropic"},
				},
			}, nil
		},
	}
	got := runCommand(t, m, "/provider unknown")
	assertLastMessageContains(t, got, "unknown")
}

func TestHandleCommandProviderRequiresLoadedConfig(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{}, errors.New("app config is not loaded")
		},
	}
	got := runCommand(t, m, "/provider openai")
	assertLastMessageContains(t, got, "not loaded")
}

func TestHandleCommandProviderWriteFailure(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{}, errors.New("write failed")
		},
	}
	got := runCommand(t, m, "/provider openai")
	assertLastMessageContains(t, got, "write failed")
}

func TestHandleCommandProviderValidationFailure(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{
				MutationFeedback: &services.MutationFeedback{
					AssistantMessage: "validation failed",
					APIKeyReady:      false,
					Snapshot:         services.UISnapshot{ProviderName: "openai", CurrentModel: "gpt-5.4"},
					ValidationErr:    errors.New("validation failed"),
				},
			}, nil
		},
	}
	got := runCommand(t, m, "/provider openai")

	if got.chat.APIKeyReady {
		t.Fatal("expected validation failure to mark API key as not ready")
	}
	assertLastMessageContains(t, got, "validation failed")
}

func TestHandleCommandProviderSuccess(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{
				MutationFeedback: &services.MutationFeedback{
					AssistantMessage: "Switched provider to openai.",
					APIKeyReady:      true,
					Snapshot:         services.UISnapshot{ProviderName: "openai", CurrentModel: "gpt-5.4"},
				},
			}, nil
		},
	}
	got := runCommand(t, m, "/provider openai")

	if !got.chat.APIKeyReady {
		t.Fatal("expected successful provider switch to keep API key ready")
	}
	if got.chat.ActiveModel == "" {
		t.Fatal("expected active model to be set")
	}
	assertLastMessageContains(t, got, "openai")
}

func TestHandleCommandSwitchModelValidationSuccess(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{
				MutationFeedback: &services.MutationFeedback{
					AssistantMessage: "Switched model to: gpt-5.4-mini",
					APIKeyReady:      true,
					Snapshot:         services.UISnapshot{CurrentModel: "gpt-5.4-mini"},
				},
			}, nil
		},
	}
	got := runCommand(t, m, "/switch gpt-5.4-mini")

	if !got.chat.APIKeyReady {
		t.Fatal("expected API key to stay ready")
	}
	if got.chat.ActiveModel != "gpt-5.4-mini" {
		t.Fatalf("expected active model to switch, got %q", got.chat.ActiveModel)
	}
}

func TestHandleCommandSwitchRequiresArgument(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)

	got := runCommand(t, m, "/switch")
	assertLastMessageContains(t, got, "/switch <model>")
}

func TestHandleCommandSwitchRequiresLoadedConfig(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{}, errors.New("app config is not loaded")
		},
	}
	got := runCommand(t, m, "/switch gpt-5.4")
	assertLastMessageContains(t, got, "not loaded")
}

func TestHandleCommandSwitchWriteFailure(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{}, errors.New("write failed")
		},
	}
	got := runCommand(t, m, "/switch gpt-5.4")
	assertLastMessageContains(t, got, "write failed")
}

func TestHandleCommandSwitchMissingRuntimeKey(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{
				MutationFeedback: &services.MutationFeedback{
					AssistantMessage: "MISSING_ENV",
					APIKeyReady:      false,
					Snapshot:         services.UISnapshot{CurrentModel: "gpt-5.4"},
					ValidationErr:    fmt.Errorf("%w: MISSING_ENV", services.ErrAPIKeyMissing),
				},
			}, nil
		},
	}
	got := runCommand(t, m, "/switch gpt-5.4")

	if got.chat.APIKeyReady {
		t.Fatal("expected API key to be not ready when runtime key is missing")
	}
	assertLastMessageContains(t, got, "MISSING_ENV")
}

func TestHandleCommandSwitchValidationFailure(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.session = &fakeSessionService{
		handleInputFn: func(_ context.Context, _ services.SessionSnapshot, _ string) (services.InputResult, error) {
			return services.InputResult{
				MutationFeedback: &services.MutationFeedback{
					AssistantMessage: "validation failed",
					APIKeyReady:      false,
					Snapshot:         services.UISnapshot{CurrentModel: "gpt-5.4"},
					ValidationErr:    errors.New("validation failed"),
				},
			}, nil
		},
	}
	got := runCommand(t, m, "/switch gpt-5.4")

	if got.chat.APIKeyReady {
		t.Fatal("expected validation failure to mark API key not ready")
	}
	assertLastMessageContains(t, got, "validation failed")
}

func TestHandleCommandWorkspaceShowsUnknownWhenRootMissing(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.WorkspaceRoot = ""

	got := runCommand(t, m, "/workspace")

	if !strings.Contains(lastMessageContent(t, got), "unknown") {
		t.Fatalf("expected unknown workspace message, got %q", lastMessageContent(t, got))
	}
}

func TestHandleCommandWorkspaceRejectsExtraArgs(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)

	got := runCommand(t, m, "/pwd extra")
	assertLastMessageContains(t, got, "/pwd")
}

func TestHandleCommandMemoryFailure(t *testing.T) {
	client := &fakeChatClient{memoryErr: errors.New("stats failed")}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true

	got := runCommand(t, m, "/memory")
	assertLastMessageContains(t, got, "stats failed")
}

func TestHandleCommandMemorySuccess(t *testing.T) {
	client := &fakeChatClient{memoryStats: &services.MemoryStats{
		PersistentItems: 1,
		SessionItems:    2,
		TotalItems:      3,
		TopK:            4,
		MinScore:        1.5,
		Path:            "memory.json",
		ByType: map[string]int{
			services.TypeUserPreference: 1,
		},
	}}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true

	got := runCommand(t, m, "/memory")
	assertLastMessageContains(t, got, "memory.json")
}

func TestHandleCommandClearMemoryRequiresConfirm(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true

	got := runCommand(t, m, "/clear-memory")
	assertLastMessageContains(t, got, "confirm")
}

func TestHandleCommandClearMemoryFailure(t *testing.T) {
	client := &fakeChatClient{clearMemoryErr: errors.New("clear failed")}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true

	got := runCommand(t, m, "/clear-memory confirm")
	assertLastMessageContains(t, got, "clear failed")
}

func TestHandleCommandClearMemorySuccessRefreshesStats(t *testing.T) {
	client := &fakeChatClient{memoryStats: &services.MemoryStats{TotalItems: 9}}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true

	got := runCommand(t, m, "/clear-memory confirm")

	if got.chat.MemoryStats.TotalItems != 9 {
		t.Fatalf("expected stats refresh, got %+v", got.chat.MemoryStats)
	}
}

func TestHandleCommandClearContextFailure(t *testing.T) {
	client := &fakeChatClient{clearSessionErr: errors.New("clear session failed")}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true

	got := runCommand(t, m, "/clear-context")
	assertLastMessageContains(t, got, "clear session failed")
}

func TestHandleCommandUnknownCommand(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true

	got := runCommand(t, m, "/unknown")
	assertLastMessageContains(t, got, "/unknown")
}

func TestStreamChunkMsgAppendsContentAndSchedulesNextChunk(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.Generating = true
	m.chat.Messages = []state.Message{{Role: "assistant", Content: ""}}

	ch := make(chan string, 1)
	ch <- "second"
	close(ch)
	m.streamChan = ch

	updated, cmd := m.Update(StreamChunkMsg{Content: "first"})
	got := updated.(Model)

	if got.chat.Messages[0].Content != "first" {
		t.Fatalf("expected first chunk to append, got %q", got.chat.Messages[0].Content)
	}
	if cmd == nil {
		t.Fatal("expected follow-up command")
	}
	msg := cmd()
	chunk, ok := msg.(StreamChunkMsg)
	if !ok {
		t.Fatalf("expected StreamChunkMsg, got %T", msg)
	}
	if chunk.Content != "second" {
		t.Fatalf("expected second chunk, got %q", chunk.Content)
	}
}

func TestWindowSizeMsgUpdatesLayout(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.ui.Width != 100 || got.ui.Height != 40 {
		t.Fatalf("expected updated size, got %dx%d", got.ui.Width, got.ui.Height)
	}
}

func TestMouseMsgUpdatesViewport(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.viewport.SetContent("line1\nline2\nline3\nline4")

	updated, _ := m.Update(tea.MouseMsg{Type: tea.MouseWheelDown})
	_ = updated.(Model)
}

func TestMouseClickCopiesCodeBlock(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.Messages = []state.Message{{Role: "assistant", Content: "```go\nfmt.Println(1)\n```"}}
	m.refreshViewport()
	if len(m.chatLayout.Regions) != 1 {
		t.Fatalf("expected one clickable region, got %d", len(m.chatLayout.Regions))
	}
	region := m.chatLayout.Regions[0]

	var copied string
	m.copyToClipboard = func(text string) error {
		copied = text
		return nil
	}

	updated, cmd := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: region.StartCol, Y: region.StartRow})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no follow-up command")
	}
	if copied != "fmt.Println(1)" {
		t.Fatalf("expected code to be copied, got %q", copied)
	}
	if !strings.Contains(got.ui.StatusMessage, "copied: go") {
		t.Fatalf("expected copy status, got %q", got.ui.StatusMessage)
	}
}

func TestMouseClickCopyFailureShowsEnglishStatus(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.Messages = []state.Message{{Role: "assistant", Content: "```go\nfmt.Println(1)\n```"}}
	m.refreshViewport()
	if len(m.chatLayout.Regions) != 1 {
		t.Fatalf("expected one clickable region, got %d", len(m.chatLayout.Regions))
	}
	region := m.chatLayout.Regions[0]

	m.copyToClipboard = func(string) error {
		return errors.New("clipboard unavailable")
	}

	updated, cmd := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: region.StartCol, Y: region.StartRow})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no follow-up command")
	}
	if !strings.Contains(got.ui.StatusMessage, "copy failed") {
		t.Fatalf("expected copy failure status, got %q", got.ui.StatusMessage)
	}
	if !strings.Contains(got.ui.LastError, "clipboard unavailable") {
		t.Fatalf("expected last error to capture copy failure, got %q", got.ui.LastError)
	}
}

func TestMouseClickOutsideCopyRegionDoesNotCopy(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.Messages = []state.Message{{Role: "assistant", Content: "```go\nfmt.Println(1)\n```"}}
	m.refreshViewport()
	initialStatus := m.ui.StatusMessage

	called := false
	m.copyToClipboard = func(string) error {
		called = true
		return nil
	}

	updated, _ := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 20, Y: 2})
	got := updated.(Model)

	if called {
		t.Fatal("expected copy not to trigger")
	}
	if got.ui.StatusMessage != initialStatus {
		t.Fatalf("expected status message to remain unchanged, got %q", got.ui.StatusMessage)
	}
}

func TestStreamChunkMsgNoOpWhenNotGenerating(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.Generating = false
	m.chat.Messages = []state.Message{{Role: "assistant", Content: "start"}}

	updated, _ := m.Update(StreamChunkMsg{Content: "ignored"})
	got := updated.(Model)

	if got.chat.Messages[0].Content != "start" {
		t.Fatalf("expected content unchanged, got %q", got.chat.Messages[0].Content)
	}
}

func TestStreamDoneMsgRequestsAssistantTurnResolution(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.Generating = true
	m.chat.Messages = []state.Message{{Role: "assistant", Content: "done", Streaming: true}}

	called := false
	m.session = &fakeSessionService{
		continueAfterStreamFn: func(_ context.Context, snapshot services.SessionSnapshot) (services.TurnResolution, error) {
			called = true
			if len(snapshot.Messages) != 1 || snapshot.Messages[0].Content != "done" {
				t.Fatalf("expected current conversation messages, got %+v", snapshot.Messages)
			}
			return services.TurnResolution{}, nil
		},
	}

	updated, cmd := m.Update(StreamDoneMsg{})
	got := updated.(Model)

	if got.chat.Generating {
		t.Fatal("expected generation to stop before resolution")
	}
	if got.streamChan != nil {
		t.Fatal("expected stream channel to be cleared")
	}
	if got.chat.Messages[0].Streaming {
		t.Fatal("expected last message streaming flag to clear")
	}
	if cmd == nil {
		t.Fatal("expected resolution command")
	}
	msg := cmd()
	if _, ok := msg.(TurnResolvedMsg); !ok {
		t.Fatalf("expected TurnResolvedMsg, got %T", msg)
	}
	if !called {
		t.Fatal("expected assistant turn resolution to be requested")
	}
}

func TestShowHideHelpRefreshMemoryAndExitMsgs(t *testing.T) {
	client := &fakeChatClient{memoryStats: &services.MemoryStats{TotalItems: 7}}
	m := newTestModel(t, client)

	updated, _ := m.Update(ShowHelpMsg{})
	got := updated.(Model)
	if got.ui.Mode != state.ModeHelp {
		t.Fatalf("expected help mode, got %v", got.ui.Mode)
	}

	updated, _ = got.Update(HideHelpMsg{})
	got = updated.(Model)
	if got.ui.Mode != state.ModeChat {
		t.Fatalf("expected chat mode, got %v", got.ui.Mode)
	}

	updated, cmd := got.Update(RefreshMemoryMsg{})
	if cmd == nil {
		t.Fatal("expected refresh memory command")
	}
	updated, _ = got.Update(cmd())
	got = updated.(Model)
	if got.chat.MemoryStats.TotalItems != 7 {
		t.Fatalf("expected refreshed stats, got %+v", got.chat.MemoryStats)
	}

	_, cmd = got.Update(ExitMsg{})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestRefreshMemoryMsgIgnoresClientError(t *testing.T) {
	client := &fakeChatClient{memoryErr: errors.New("stats failed")}
	m := newTestModel(t, client)
	m.chat.MemoryStats.TotalItems = 5

	updated, cmd := m.Update(RefreshMemoryMsg{})
	if cmd == nil {
		t.Fatal("expected refresh memory command")
	}
	gotModel := updated.(Model)
	updated, _ = gotModel.Update(cmd())
	got := updated.(Model)
	if got.chat.MemoryStats.TotalItems != 5 {
		t.Fatalf("expected previous stats to be preserved, got %+v", got.chat.MemoryStats)
	}
}

func TestTurnResolvedMsgAddsContextAndRestartsStreaming(t *testing.T) {
	client := &fakeChatClient{chatChunks: []string{"tool follow-up"}}
	m := newTestModel(t, client)
	m.chat.Messages = []state.Message{{Role: "user", Content: "hello"}}

	stream, err := client.Chat(context.Background(), []services.Message{{Role: "user", Content: "hello"}}, "")
	if err != nil {
		t.Fatalf("expected fake stream, got %v", err)
	}

	updated, cmd := m.Update(TurnResolvedMsg{Resolution: services.TurnResolution{
		Messages: []services.SessionMessage{
			services.FormatToolContextMessage(&services.ToolResult{ToolName: "read", Success: true, Output: "README"}),
		},
		StatusMessage: "Generating...",
		Stream:        stream,
	}})
	got := updated.(Model)

	if !got.chat.Generating {
		t.Fatal("expected follow-up generation to start")
	}
	if len(got.chat.Messages) != 3 {
		t.Fatalf("expected tool context and placeholder messages, got %d", len(got.chat.Messages))
	}
	if got.chat.Messages[1].Kind != services.MessageKindToolContext {
		t.Fatalf("expected tool context kind, got %+v", got.chat.Messages[1])
	}
	if got.chat.Messages[2].Role != "assistant" || got.chat.Messages[2].Content != "" {
		t.Fatalf("expected assistant placeholder, got %+v", got.chat.Messages[2])
	}
	if cmd == nil {
		t.Fatal("expected streaming command")
	}
	msg := cmd()
	chunk, ok := msg.(StreamChunkMsg)
	if !ok {
		t.Fatalf("expected StreamChunkMsg, got %T", msg)
	}
	if chunk.Content != "tool follow-up" {
		t.Fatalf("expected tool follow-up chunk, got %q", chunk.Content)
	}
}

func TestBuildMessagesSkipsEmptyAssistantPlaceholder(t *testing.T) {
	m := Model{
		chat: state.ChatState{Messages: []state.Message{
			{Role: "system", Content: "persona"},
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: ""},
		}},
	}

	got := services.BuildRequestMessages(m.sessionMessages())
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	if got[0].Role != "system" || got[1].Role != "user" {
		t.Fatalf("unexpected message order: %+v", got)
	}
	if got[1].Content != "hello" {
		t.Fatalf("expected user message to be preserved, got %+v", got[1])
	}
}

func TestFormatTypeStats(t *testing.T) {
	if got := services.FormatMemoryTypeStats(nil); got == "" {
		t.Fatal("expected non-empty placeholder")
	}
	got := services.FormatMemoryTypeStats(map[string]int{
		services.TypeUserPreference: 2,
		services.TypeCodeFact:       1,
	})
	if !strings.Contains(got, services.TypeUserPreference+"=2") || !strings.Contains(got, services.TypeCodeFact+"=1") {
		t.Fatalf("unexpected formatted stats %q", got)
	}
}

func TestRecentToolContextIndexes(t *testing.T) {
	messages := []services.SessionMessage{
		{Role: "system", Content: "Tool result\na", Kind: services.MessageKindToolContext},
		{Role: "assistant", Content: "x"},
		{Role: "system", Content: "Tool result\nb", Kind: services.MessageKindToolContext},
	}
	got := services.RecentToolContextIndexes(messages, 1)
	if len(got) != 1 {
		t.Fatalf("expected one index, got %+v", got)
	}
	if _, ok := got[2]; !ok {
		t.Fatalf("expected newest index to be kept, got %+v", got)
	}
}

func TestFormatToolStatusMessage(t *testing.T) {
	got := services.FormatToolStatusMessage("read", map[string]interface{}{"filePath": "README.md"}).Content
	if !strings.Contains(got, "tool=read") || !strings.Contains(got, "README.md") {
		t.Fatalf("unexpected tool status %q", got)
	}
}

func TestFormatToolContextMessage(t *testing.T) {
	got := services.FormatToolContextMessage(&services.ToolResult{
		ToolName: "read",
		Success:  true,
		Output:   "hello",
		Metadata: map[string]interface{}{"k": "v"},
	}).Content
	if !strings.Contains(got, "tool=read") || !strings.Contains(got, "metadata=") || !strings.Contains(got, "output:") {
		t.Fatalf("unexpected tool context %q", got)
	}

	got = services.FormatToolContextMessage(&services.ToolResult{ToolName: "read", Success: false, Error: "boom"}).Content
	if !strings.Contains(got, "error:") || !strings.Contains(got, "boom") {
		t.Fatalf("unexpected error context %q", got)
	}
}

func TestFormatToolErrorContext(t *testing.T) {
	got := services.FormatToolErrorContext(errors.New("boom")).Content
	if !strings.Contains(got, "boom") {
		t.Fatalf("unexpected tool error context %q", got)
	}
}

func TestTruncateForContext(t *testing.T) {
	if got := services.TruncateForContext("  hi  ", 10); got != "hi" {
		t.Fatalf("expected trimmed content, got %q", got)
	}
	got := services.TruncateForContext(strings.Repeat("a", 20), 10)
	if !strings.Contains(got, "truncated") {
		t.Fatalf("expected truncation marker, got %q", got)
	}
}
func TestCalculateInputHeight(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)

	m.textarea.SetValue("one")
	if got := m.calculateInputHeight(); got != 3 {
		t.Fatalf("expected minimum height 3, got %d", got)
	}
	m.textarea.SetValue(strings.Repeat("a\n", 10))
	if got := m.calculateInputHeight(); got != 8 {
		t.Fatalf("expected capped height 8, got %d", got)
	}
}

func TestHandleInputErrorReturnsAssistantErrorMessage(t *testing.T) {
	m := NewModel(&fakeSessionService{
		handleInputFn: func(context.Context, services.SessionSnapshot, string) (services.InputResult, error) {
			return services.InputResult{}, errors.New("chat failed")
		},
	}, "")

	updated, cmd := m.handleCommand("/memory")
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	updated, _ = updated.(Model).Update(msg)
	got := updated.(Model)
	if len(got.chat.Messages) != 1 {
		t.Fatalf("expected one assistant error, got %d", len(got.chat.Messages))
	}
	if !strings.Contains(got.chat.Messages[0].Content, "chat failed") {
		t.Fatalf("expected error message, got %q", got.chat.Messages[0].Content)
	}
}

func TestStreamResponseFromChannelDone(t *testing.T) {
	m := Model{}
	ch := make(chan string)
	close(ch)
	m.streamChan = ch

	msg := m.streamResponseFromChannel()()
	if _, ok := msg.(StreamDoneMsg); !ok {
		t.Fatalf("expected StreamDoneMsg after empty channel, got %T", msg)
	}

	m.streamChan = nil
	if cmd := m.streamResponseFromChannel(); cmd != nil {
		t.Fatal("expected nil command when stream channel is nil")
	}
}

func TestStreamErrorReplacesTrailingPlaceholder(t *testing.T) {
	m := Model{
		chat: state.ChatState{
			Messages: []state.Message{
				{Role: "user", Content: "hello"},
				{Role: "assistant", Content: ""},
			},
		},
	}

	updated, _ := m.Update(StreamErrorMsg{Err: errors.New("boom")})
	got := updated.(Model)
	if len(got.chat.Messages) != 2 {
		t.Fatalf("expected placeholder replacement without extra message, got %d messages", len(got.chat.Messages))
	}
	if !strings.Contains(got.chat.Messages[1].Content, "boom") {
		t.Fatalf("expected trailing placeholder to become error, got %q", got.chat.Messages[1].Content)
	}
}

func TestClearContextDoesNotReinjectStalePersonaMessage(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true
	m.chat.Messages = []state.Message{
		{Role: "system", Content: "stale persona"},
		{Role: "user", Content: "hello"},
	}

	got := runCommand(t, m, "/clear-context")
	if len(got.chat.Messages) != 1 {
		t.Fatalf("expected only confirmation message after clear-context, got %d messages", len(got.chat.Messages))
	}
	if got.chat.Messages[0].Role != "assistant" {
		t.Fatalf("expected confirmation assistant message, got %+v", got.chat.Messages[0])
	}
}

func TestBuildMessagesSkipsTransientToolStatusMessage(t *testing.T) {
	m := Model{
		chat: state.ChatState{Messages: []state.Message{
			{Role: "user", Content: "hello"},
			{Role: "system", Content: "Tool status\ntool=read\nfile=README.md", Kind: services.MessageKindToolStatus},
			{Role: "assistant", Content: "ok"},
		}},
	}

	got := services.BuildRequestMessages(m.sessionMessages())
	if len(got) != 2 {
		t.Fatalf("expected 2 messages after filtering tool status, got %d", len(got))
	}
	for _, msg := range got {
		if msg.Role == "system" && strings.Contains(msg.Content, "Tool status") {
			t.Fatalf("transient tool status should not be included in model context: %+v", msg)
		}
	}
}

func TestBuildMessagesKeepsOnlyRecentToolContextMessages(t *testing.T) {
	config.GlobalAppConfig = config.DefaultAppConfig()
	m := Model{}
	m.chat.Messages = append(m.chat.Messages, state.Message{Role: "user", Content: "step 1"})
	for i := 1; i <= 5; i++ {
		m.chat.Messages = append(m.chat.Messages, state.Message{
			Role:    "system",
			Content: "Tool result\ntool=read\nsuccess=true\noutput:\nchunk " + string(rune('0'+i)),
			Kind:    services.MessageKindToolContext,
		})
	}
	m.chat.Messages = append(m.chat.Messages, state.Message{Role: "assistant", Content: "done"})

	got := services.BuildRequestMessages(m.sessionMessages())
	toolCtxCount := 0
	for _, msg := range got {
		if msg.Role == "system" && strings.HasPrefix(msg.Content, "Tool result") {
			toolCtxCount++
		}
	}
	if toolCtxCount != config.GlobalAppConfig.History.MaxToolContextMessages {
		t.Fatalf("expected %d tool context messages, got %d", config.GlobalAppConfig.History.MaxToolContextMessages, toolCtxCount)
	}

	joined := ""
	for _, msg := range got {
		joined += msg.Content + "\n"
	}
	if strings.Contains(joined, "chunk 1") || strings.Contains(joined, "chunk 2") {
		t.Fatalf("old tool context should be evicted, got context: %s", joined)
	}
	if !strings.Contains(joined, "chunk 3") || !strings.Contains(joined, "chunk 4") || !strings.Contains(joined, "chunk 5") {
		t.Fatalf("newest tool context messages should be kept, got context: %s", joined)
	}
}

func TestWorkspaceCommandShowsWorkspaceRoot(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true
	m.chat.WorkspaceRoot = `F:/Qiniu/test1`

	got := runCommand(t, m, "/pwd")
	if len(got.chat.Messages) != 1 {
		t.Fatalf("expected exactly 1 message, got %d", len(got.chat.Messages))
	}
	if got.chat.Messages[0].Role != "assistant" {
		t.Fatalf("expected assistant message, got %+v", got.chat.Messages[0])
	}
	if !strings.Contains(got.chat.Messages[0].Content, `F:/Qiniu/test1`) {
		t.Fatalf("expected workspace path in response, got %q", got.chat.Messages[0].Content)
	}
}

func TestApproveCommandWhileApprovalRunningKeepsPendingApproval(t *testing.T) {
	m := Model{
		session: services.NewSessionService(services.NewRuntimeController(&fakeChatClient{}, "config.yaml")),
		ui: state.UIState{
			ApprovalRunning: true,
		},
		chat: state.ChatState{
			PendingApproval: &state.PendingApproval{
				Call: services.ToolCall{
					Tool: "bash",
					Params: map[string]interface{}{
						"command": "echo hello",
					},
				},
				ToolType: "Bash",
				Target:   "echo hello",
			},
		},
	}

	updated, cmd := m.handleCommand("/y")
	if cmd == nil {
		t.Fatal("expected command")
	}
	updated, _ = updated.(Model).Update(cmd())

	got := updated.(Model)
	if got.chat.PendingApproval == nil {
		t.Fatal("expected pending approval to be preserved")
	}
	if got.chat.PendingApproval.Call.Tool != "bash" {
		t.Fatalf("expected pending tool to stay intact, got %+v", got.chat.PendingApproval.Call)
	}
	if len(got.chat.Messages) != 1 {
		t.Fatalf("expected a single assistant warning, got %d", len(got.chat.Messages))
	}
	if !strings.Contains(got.chat.Messages[0].Content, "/y") {
		t.Fatalf("expected warning message to mention /y, got %q", got.chat.Messages[0].Content)
	}
}

func TestTurnResolvedMsgStoresPendingApproval(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)

	updated, cmd := m.Update(TurnResolvedMsg{Resolution: services.TurnResolution{
		PendingApproval: &services.ToolApprovalRequest{
			Call: services.ToolCall{
				Tool: "bash",
				Params: map[string]interface{}{
					"command": "echo hello",
				},
			},
			ToolType:         "Bash",
			Target:           "echo hello",
			AssistantMessage: "Use /y to approve.",
		},
		Messages: []services.SessionMessage{
			{Role: "assistant", Content: "Use /y to approve.", Transient: true},
		},
	}})
	if cmd != nil {
		t.Fatal("expected no follow-up command while waiting for approval")
	}

	got := updated.(Model)
	if got.chat.PendingApproval == nil {
		t.Fatal("expected pending approval to be recorded")
	}
	if got.chat.PendingApproval.Target != "echo hello" {
		t.Fatalf("unexpected pending approval target %q", got.chat.PendingApproval.Target)
	}
	if len(got.chat.Messages) != 1 {
		t.Fatalf("expected one approval prompt message, got %d", len(got.chat.Messages))
	}
	if !strings.Contains(got.chat.Messages[0].Content, "/y") {
		t.Fatalf("expected approval prompt to mention /y, got %q", got.chat.Messages[0].Content)
	}
}
