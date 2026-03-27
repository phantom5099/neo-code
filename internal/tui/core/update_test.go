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

func newTestModel(t *testing.T, client *fakeChatClient) *Model {
	t.Helper()

	restoreCoreGlobals(t)
	cfg := config.DefaultAppConfig()
	config.GlobalAppConfig = cfg

	m := NewModel(client, 4, "config.yaml", "")
	m.ui.Width = 80
	m.ui.Height = 24
	m.syncLayout()
	return &m
}

func restoreCoreGlobals(t *testing.T) {
	t.Helper()

	origGetWorkspaceRoot := getWorkspaceRoot
	origParseAssistantTools := parseAssistantTools
	origExecuteToolCall := executeToolCall
	origApproveToolCall := approveToolCall
	origUpdateAPIKeyEnvVar := updateAPIKeyEnvVar
	origSwitchProviderConfig := switchProviderConfig
	origSwitchModelConfig := switchModelConfig
	origGlobalConfig := config.GlobalAppConfig

	t.Cleanup(func() {
		getWorkspaceRoot = origGetWorkspaceRoot
		parseAssistantTools = origParseAssistantTools
		executeToolCall = origExecuteToolCall
		approveToolCall = origApproveToolCall
		updateAPIKeyEnvVar = origUpdateAPIKeyEnvVar
		switchProviderConfig = origSwitchProviderConfig
		switchModelConfig = origSwitchModelConfig
		config.GlobalAppConfig = origGlobalConfig
	})
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

	if cmd != nil {
		t.Fatal("expected no command when API key is not ready")
	}
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
	if !got.chat.Generating {
		t.Fatal("expected generating=true")
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
	if len(got.chat.CommandHistory) != 1 || got.chat.CommandHistory[0] != "hello" {
		t.Fatalf("expected command history to record input, got %+v", got.chat.CommandHistory)
	}

	msg := cmd()
	chunk, ok := msg.(StreamChunkMsg)
	if !ok {
		t.Fatalf("expected StreamChunkMsg, got %T", msg)
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

	updated, cmd := m.handleCommand("/memory")
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no command")
	}
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

	updated, _ := m.handleCommand("/apikey")
	got := updated.(Model)
	assertLastMessageContains(t, got, "/apikey <env_name>")
}

func TestHandleCommandAPIKeyRequiresLoadedConfig(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	updateAPIKeyEnvVar = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return nil, errors.New("app config is not loaded")
	}

	updated, _ := m.handleCommand("/apikey TEST_ENV")
	got := updated.(Model)
	assertLastMessageContains(t, got, "not loaded")
}

func TestHandleCommandAPIKeyEnvStillMissing(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	updateAPIKeyEnvVar = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return &services.ConfigMutationResult{
			APIKeyEnvVar: "MISSING_ENV",
		}, nil
	}

	updated, _ := m.handleCommand("/apikey MISSING_ENV")
	got := updated.(Model)

	if got.chat.APIKeyReady {
		t.Fatal("expected API key to remain not ready")
	}
	assertLastMessageContains(t, got, "MISSING_ENV")
}

func TestHandleCommandAPIKeyInvalidKey(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	updateAPIKeyEnvVar = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return &services.ConfigMutationResult{
			APIKeyEnvVar:  "BAD_ENV",
			ValidationErr: services.ErrInvalidAPIKey,
		}, nil
	}

	updated, _ := m.handleCommand("/apikey BAD_ENV")
	got := updated.(Model)

	if got.chat.APIKeyReady {
		t.Fatal("expected invalid key to mark API key as not ready")
	}
	assertLastMessageContains(t, got, "BAD_ENV")
}

func TestHandleCommandAPIKeyGenericValidationError(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	updateAPIKeyEnvVar = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return &services.ConfigMutationResult{
			APIKeyEnvVar:  "GENERIC_ENV",
			ValidationErr: errors.New("validation failed"),
		}, nil
	}

	updated, _ := m.handleCommand("/apikey GENERIC_ENV")
	got := updated.(Model)

	if got.chat.APIKeyReady {
		t.Fatal("expected generic validation failure to mark API key as not ready")
	}
	assertLastMessageContains(t, got, "validation failed")
}

func TestHandleCommandAPIKeySuccessWritesConfig(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	updateAPIKeyEnvVar = func(_ context.Context, path, envName string) (*services.ConfigMutationResult, error) {
		if path != "config.yaml" {
			t.Fatalf("expected config path config.yaml, got %q", path)
		}
		if envName != "TEST_API_KEY_ENV" {
			t.Fatalf("expected env name TEST_API_KEY_ENV, got %q", envName)
		}
		return &services.ConfigMutationResult{
			APIKeyEnvVar: "TEST_API_KEY_ENV",
			APIKeyReady:  true,
		}, nil
	}

	updated, cmd := m.handleCommand("/apikey TEST_API_KEY_ENV")
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no command")
	}
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
	updateAPIKeyEnvVar = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return nil, errors.New("write failed")
	}

	updated, _ := m.handleCommand("/apikey NEW_ENV")
	got := updated.(Model)

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
	switchProviderConfig = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return &services.ConfigMutationResult{
			Provider:      "openai",
			Model:         "gpt-5.4",
			APIKeyEnvVar:  "MISSING_ENV",
			ValidationErr: fmt.Errorf("%w: MISSING_ENV", services.ErrAPIKeyMissing),
		}, nil
	}

	updated, _ := m.handleCommand("/provider openai")
	got := updated.(Model)

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

	updated, _ := m.handleCommand("/provider")
	got := updated.(Model)
	assertLastMessageContains(t, got, "/provider <name>")
}

func TestHandleCommandProviderRejectsUnknownProvider(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	switchProviderConfig = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return nil, errors.New("unsupported provider")
	}

	updated, _ := m.handleCommand("/provider unknown")
	got := updated.(Model)
	assertLastMessageContains(t, got, "unknown")
}

func TestHandleCommandProviderRequiresLoadedConfig(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	switchProviderConfig = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return nil, errors.New("app config is not loaded")
	}

	updated, _ := m.handleCommand("/provider openai")
	got := updated.(Model)
	assertLastMessageContains(t, got, "not loaded")
}

func TestHandleCommandProviderWriteFailure(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	switchProviderConfig = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return nil, errors.New("write failed")
	}

	updated, _ := m.handleCommand("/provider openai")
	got := updated.(Model)
	assertLastMessageContains(t, got, "write failed")
}

func TestHandleCommandProviderValidationFailure(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	switchProviderConfig = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return &services.ConfigMutationResult{
			Provider:      "openai",
			Model:         "gpt-5.4",
			ValidationErr: errors.New("validation failed"),
		}, nil
	}

	updated, _ := m.handleCommand("/provider openai")
	got := updated.(Model)

	if got.chat.APIKeyReady {
		t.Fatal("expected validation failure to mark API key as not ready")
	}
	assertLastMessageContains(t, got, "validation failed")
}

func TestHandleCommandProviderSuccess(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	switchProviderConfig = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return &services.ConfigMutationResult{
			Provider:    "openai",
			Model:       "gpt-5.4",
			APIKeyReady: true,
		}, nil
	}

	updated, _ := m.handleCommand("/provider openai")
	got := updated.(Model)

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
	switchModelConfig = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return &services.ConfigMutationResult{
			Model:       "gpt-5.4-mini",
			APIKeyReady: true,
		}, nil
	}

	updated, _ := m.handleCommand("/switch gpt-5.4-mini")
	got := updated.(Model)

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

	updated, _ := m.handleCommand("/switch")
	got := updated.(Model)
	assertLastMessageContains(t, got, "/switch <model>")
}

func TestHandleCommandSwitchRequiresLoadedConfig(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	switchModelConfig = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return nil, errors.New("app config is not loaded")
	}

	updated, _ := m.handleCommand("/switch gpt-5.4")
	got := updated.(Model)
	assertLastMessageContains(t, got, "not loaded")
}

func TestHandleCommandSwitchWriteFailure(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	switchModelConfig = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return nil, errors.New("write failed")
	}

	updated, _ := m.handleCommand("/switch gpt-5.4")
	got := updated.(Model)
	assertLastMessageContains(t, got, "write failed")
}

func TestHandleCommandSwitchMissingRuntimeKey(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	switchModelConfig = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return &services.ConfigMutationResult{
			Model:         "gpt-5.4",
			APIKeyEnvVar:  "MISSING_ENV",
			ValidationErr: fmt.Errorf("%w: MISSING_ENV", services.ErrAPIKeyMissing),
		}, nil
	}

	updated, _ := m.handleCommand("/switch gpt-5.4")
	got := updated.(Model)

	if got.chat.APIKeyReady {
		t.Fatal("expected API key to be not ready when runtime key is missing")
	}
	assertLastMessageContains(t, got, "MISSING_ENV")
}

func TestHandleCommandSwitchValidationFailure(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	switchModelConfig = func(context.Context, string, string) (*services.ConfigMutationResult, error) {
		return &services.ConfigMutationResult{
			Model:         "gpt-5.4",
			ValidationErr: errors.New("validation failed"),
		}, nil
	}

	updated, _ := m.handleCommand("/switch gpt-5.4")
	got := updated.(Model)

	if got.chat.APIKeyReady {
		t.Fatal("expected validation failure to mark API key not ready")
	}
	assertLastMessageContains(t, got, "validation failed")
}

func TestHandleCommandWorkspaceFallsBackToGlobalRoot(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.WorkspaceRoot = ""
	getWorkspaceRoot = func() string { return `D:/neo-code/workspace` }

	updated, _ := m.handleCommand("/workspace")
	got := updated.(Model)

	if !strings.Contains(lastMessageContent(t, got), `D:/neo-code/workspace`) {
		t.Fatalf("expected workspace fallback path, got %q", lastMessageContent(t, got))
	}
}

func TestHandleCommandWorkspaceRejectsExtraArgs(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)

	updated, _ := m.handleCommand("/pwd extra")
	got := updated.(Model)
	assertLastMessageContains(t, got, "/pwd")
}

func TestHandleCommandMemoryFailure(t *testing.T) {
	client := &fakeChatClient{memoryErr: errors.New("stats failed")}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true

	updated, _ := m.handleCommand("/memory")
	got := updated.(Model)
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

	updated, _ := m.handleCommand("/memory")
	got := updated.(Model)
	assertLastMessageContains(t, got, "memory.json")
}

func TestHandleCommandClearMemoryRequiresConfirm(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true

	updated, _ := m.handleCommand("/clear-memory")
	got := updated.(Model)
	assertLastMessageContains(t, got, "confirm")
}

func TestHandleCommandClearMemoryFailure(t *testing.T) {
	client := &fakeChatClient{clearMemoryErr: errors.New("clear failed")}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true

	updated, _ := m.handleCommand("/clear-memory confirm")
	got := updated.(Model)
	assertLastMessageContains(t, got, "clear failed")
}

func TestHandleCommandClearMemorySuccessRefreshesStats(t *testing.T) {
	client := &fakeChatClient{memoryStats: &services.MemoryStats{TotalItems: 9}}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true

	updated, _ := m.handleCommand("/clear-memory confirm")
	got := updated.(Model)

	if got.chat.MemoryStats.TotalItems != 9 {
		t.Fatalf("expected stats refresh, got %+v", got.chat.MemoryStats)
	}
}

func TestHandleCommandClearContextFailure(t *testing.T) {
	client := &fakeChatClient{clearSessionErr: errors.New("clear session failed")}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true

	updated, _ := m.handleCommand("/clear-context")
	got := updated.(Model)
	assertLastMessageContains(t, got, "clear session failed")
}

func TestHandleCommandUnknownCommand(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.APIKeyReady = true

	updated, _ := m.handleCommand("/unknown")
	got := updated.(Model)
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

func TestStreamDoneMsgCompletesWithoutToolCall(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.Generating = true
	ch := make(chan string)
	close(ch)
	m.streamChan = ch
	m.chat.Messages = []state.Message{{Role: "assistant", Content: "done", Streaming: true}}

	updated, cmd := m.Update(StreamDoneMsg{})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.chat.Generating {
		t.Fatal("expected generation to stop")
	}
	if got.streamChan != nil {
		t.Fatal("expected stream channel to be cleared")
	}
	if got.chat.Messages[0].Streaming {
		t.Fatal("expected last message streaming flag to clear")
	}
}

func TestStreamDoneMsgDoesNotReexecuteWhenToolAlreadyExecuting(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.Generating = true
	m.chat.ToolExecuting = true
	m.chat.Messages = []state.Message{{Role: "assistant", Content: `{"tool":"read","params":{"path":"README.md"}}`, Streaming: true}}

	called := false
	executeToolCall = func(services.ToolCall) *services.ToolResult {
		called = true
		return nil
	}

	updated, cmd := m.Update(StreamDoneMsg{})
	got := updated.(Model)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if !got.chat.ToolExecuting {
		t.Fatal("expected tool executing flag to remain true")
	}
	if called {
		t.Fatal("expected no duplicate tool execution")
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

	updated, _ = got.Update(RefreshMemoryMsg{})
	got = updated.(Model)
	if got.chat.MemoryStats.TotalItems != 7 {
		t.Fatalf("expected refreshed stats, got %+v", got.chat.MemoryStats)
	}

	_, cmd := got.Update(ExitMsg{})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestRefreshMemoryMsgIgnoresClientError(t *testing.T) {
	client := &fakeChatClient{memoryErr: errors.New("stats failed")}
	m := newTestModel(t, client)
	m.chat.MemoryStats.TotalItems = 5

	updated, _ := m.Update(RefreshMemoryMsg{})
	got := updated.(Model)
	if got.chat.MemoryStats.TotalItems != 5 {
		t.Fatalf("expected previous stats to be preserved, got %+v", got.chat.MemoryStats)
	}
}

func TestStreamDoneMsgExecutesToolCallFromAssistantJSON(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.Generating = true
	m.chat.Messages = []state.Message{{Role: "assistant", Content: `{"tool":"read","params":{"path":"README.md"}}`, Streaming: true}}

	expected := &services.ToolResult{ToolName: "read", Success: true, Output: "ok"}
	executeToolCall = func(call services.ToolCall) *services.ToolResult {
		if call.Tool != "read" {
			t.Fatalf("expected read tool, got %q", call.Tool)
		}
		if got, _ := call.Params["path"].(string); got != "README.md" {
			t.Fatalf("expected normalized path param, got %+v", call.Params)
		}
		return expected
	}

	updated, cmd := m.Update(StreamDoneMsg{})
	got := updated.(Model)

	if !got.chat.ToolExecuting {
		t.Fatal("expected tool execution flag to be set")
	}
	if len(got.chat.Messages) != 2 {
		t.Fatalf("expected tool status message to be appended, got %d messages", len(got.chat.Messages))
	}
	if !strings.HasPrefix(got.chat.Messages[1].Content, toolStatusPrefix) {
		t.Fatalf("expected transient tool status, got %q", got.chat.Messages[1].Content)
	}
	if cmd == nil {
		t.Fatal("expected tool execution command")
	}
	msg := cmd()
	resultMsg, ok := msg.(ToolResultMsg)
	if !ok {
		t.Fatalf("expected ToolResultMsg, got %T", msg)
	}
	if resultMsg.Result != expected {
		t.Fatalf("expected tool result to round-trip, got %+v", resultMsg.Result)
	}
}

func TestStreamDoneMsgReturnsToolErrorWhenToolResultIsNil(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.Generating = true
	m.chat.Messages = []state.Message{{Role: "assistant", Content: `{"tool":"read","params":{"path":"README.md"}}`, Streaming: true}}

	executeToolCall = func(services.ToolCall) *services.ToolResult { return nil }

	_, cmd := m.Update(StreamDoneMsg{})
	if cmd == nil {
		t.Fatal("expected tool execution command")
	}
	msg := cmd()
	if _, ok := msg.(ToolErrorMsg); !ok {
		t.Fatalf("expected ToolErrorMsg, got %T", msg)
	}
}

func TestToolResultMsgAddsContextAndRestartsStreaming(t *testing.T) {
	client := &fakeChatClient{chatChunks: []string{"tool follow-up"}}
	m := newTestModel(t, client)
	m.chat.Messages = []state.Message{{Role: "user", Content: "hello"}}
	m.chat.ToolExecuting = true

	result := &services.ToolResult{ToolName: "read", Success: true, Output: "README"}
	updated, cmd := m.Update(ToolResultMsg{Result: result})
	got := updated.(Model)

	if got.chat.ToolExecuting {
		t.Fatal("expected tool execution flag to be cleared")
	}
	if !got.chat.Generating {
		t.Fatal("expected follow-up generation to start")
	}
	if len(got.chat.Messages) != 3 {
		t.Fatalf("expected tool context and placeholder messages, got %d", len(got.chat.Messages))
	}
	if !strings.HasPrefix(got.chat.Messages[1].Content, toolContextPrefix) {
		t.Fatalf("expected tool context message, got %q", got.chat.Messages[1].Content)
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

func TestToolErrorMsgAddsErrorContextAndRestartsStreaming(t *testing.T) {
	client := &fakeChatClient{chatChunks: []string{"error recovery"}}
	m := newTestModel(t, client)
	m.chat.ToolExecuting = true

	updated, cmd := m.Update(ToolErrorMsg{Err: errors.New("tool failed")})
	got := updated.(Model)

	if got.chat.ToolExecuting {
		t.Fatal("expected tool execution flag to be cleared")
	}
	if !got.chat.Generating {
		t.Fatal("expected generation restart after tool error")
	}
	if len(got.chat.Messages) != 2 {
		t.Fatalf("expected tool error context and placeholder, got %d messages", len(got.chat.Messages))
	}
	if !strings.Contains(got.chat.Messages[0].Content, "tool failed") {
		t.Fatalf("expected tool error context, got %q", got.chat.Messages[0].Content)
	}
	if cmd == nil {
		t.Fatal("expected follow-up stream command")
	}
	msg := cmd()
	if _, ok := msg.(StreamChunkMsg); !ok {
		t.Fatalf("expected StreamChunkMsg, got %T", msg)
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

	got := m.buildMessages()
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
	if got := formatTypeStats(nil); got == "" {
		t.Fatal("expected non-empty placeholder")
	}
	got := formatTypeStats(map[string]int{
		services.TypeUserPreference: 2,
		services.TypeCodeFact:       1,
	})
	if !strings.Contains(got, services.TypeUserPreference+"=2") || !strings.Contains(got, services.TypeCodeFact+"=1") {
		t.Fatalf("unexpected formatted stats %q", got)
	}
}

func TestRecentToolContextIndexes(t *testing.T) {
	messages := []state.Message{
		{Role: "system", Content: "[TOOL_CONTEXT]\na"},
		{Role: "assistant", Content: "x"},
		{Role: "system", Content: "[TOOL_CONTEXT]\nb"},
	}
	got := recentToolContextIndexes(messages, 1)
	if len(got) != 1 {
		t.Fatalf("expected one index, got %+v", got)
	}
	if _, ok := got[2]; !ok {
		t.Fatalf("expected newest index to be kept, got %+v", got)
	}
}

func TestFormatToolStatusMessage(t *testing.T) {
	got := formatToolStatusMessage("read", map[string]interface{}{"filePath": "README.md"})
	if !strings.Contains(got, "tool=read") || !strings.Contains(got, "README.md") {
		t.Fatalf("unexpected tool status %q", got)
	}
}

func TestFormatToolContextMessage(t *testing.T) {
	got := formatToolContextMessage(&services.ToolResult{
		ToolName: "read",
		Success:  true,
		Output:   "hello",
		Metadata: map[string]interface{}{"k": "v"},
	})
	if !strings.Contains(got, "tool=read") || !strings.Contains(got, "metadata=") || !strings.Contains(got, "output:") {
		t.Fatalf("unexpected tool context %q", got)
	}

	got = formatToolContextMessage(&services.ToolResult{ToolName: "read", Success: false, Error: "boom"})
	if !strings.Contains(got, "error:") || !strings.Contains(got, "boom") {
		t.Fatalf("unexpected error context %q", got)
	}
}

func TestFormatToolErrorContext(t *testing.T) {
	got := formatToolErrorContext(errors.New("boom"))
	if !strings.Contains(got, "boom") {
		t.Fatalf("unexpected tool error context %q", got)
	}
}

func TestTruncateForContext(t *testing.T) {
	if got := truncateForContext("  hi  ", 10); got != "hi" {
		t.Fatalf("expected trimmed content, got %q", got)
	}
	got := truncateForContext(strings.Repeat("a", 20), 10)
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

func TestStreamResponseReturnsErrorMsg(t *testing.T) {
	client := &fakeChatClient{chatErr: errors.New("chat failed")}
	m := newTestModel(t, client)

	cmd := m.streamResponse([]services.Message{{Role: "user", Content: "hi"}})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if _, ok := msg.(StreamErrorMsg); !ok {
		t.Fatalf("expected StreamErrorMsg, got %T", msg)
	}
}

func TestStreamResponseAndStreamResponseFromChannelDone(t *testing.T) {
	client := &fakeChatClient{chatChunks: nil}
	m := newTestModel(t, client)

	cmd := m.streamResponse([]services.Message{{Role: "user", Content: "hi"}})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if _, ok := msg.(StreamDoneMsg); !ok {
		t.Fatalf("expected StreamDoneMsg, got %T", msg)
	}

	m.streamChan = nil
	if cmd := m.streamResponseFromChannel(); cmd != nil {
		t.Fatal("expected nil command when stream channel is nil")
	}
}

func TestStreamErrorReplacesTrailingPlaceholder(t *testing.T) {
	m := Model{
		chat: state.ChatState{
			HistoryTurns: 6,
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

	updated, _ := m.handleCommand("/clear-context")
	got := updated.(Model)
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
			{Role: "system", Content: "[TOOL_STATUS] tool=read file=README.md"},
			{Role: "assistant", Content: "ok"},
		}},
	}

	got := m.buildMessages()
	if len(got) != 2 {
		t.Fatalf("expected 2 messages after filtering tool status, got %d", len(got))
	}
	for _, msg := range got {
		if msg.Role == "system" && strings.HasPrefix(msg.Content, "[TOOL_STATUS]") {
			t.Fatalf("transient tool status should not be included in model context: %+v", msg)
		}
	}
}

func TestBuildMessagesKeepsOnlyRecentToolContextMessages(t *testing.T) {
	m := Model{}
	m.chat.Messages = append(m.chat.Messages, state.Message{Role: "user", Content: "step 1"})
	for i := 1; i <= 5; i++ {
		m.chat.Messages = append(m.chat.Messages, state.Message{Role: "system", Content: "[TOOL_CONTEXT]\ntool=read\nsuccess=true\noutput:\nchunk " + string(rune('0'+i))})
	}
	m.chat.Messages = append(m.chat.Messages, state.Message{Role: "assistant", Content: "done"})

	got := m.buildMessages()
	toolCtxCount := 0
	for _, msg := range got {
		if msg.Role == "system" && strings.HasPrefix(msg.Content, "[TOOL_CONTEXT]") {
			toolCtxCount++
		}
	}
	if toolCtxCount != maxToolContextMessages {
		t.Fatalf("expected %d tool context messages, got %d", maxToolContextMessages, toolCtxCount)
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

	updated, _ := m.handleCommand("/pwd")
	got := updated.(Model)
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

func TestApproveCommandWhileToolExecutingKeepsPendingApproval(t *testing.T) {
	m := Model{
		chat: state.ChatState{
			ToolExecuting: true,
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
	if cmd != nil {
		t.Fatal("expected no tool execution command while another tool is running")
	}

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

func TestToolResultMsgSecurityAskStoresPendingApproval(t *testing.T) {
	client := &fakeChatClient{}
	m := newTestModel(t, client)
	m.chat.ToolExecuting = true

	result := &services.ToolResult{
		ToolName: "bash",
		Success:  false,
		Metadata: map[string]interface{}{
			"securityAction":   "ask",
			"securityToolType": "Bash",
			"securityTarget":   "echo hello",
		},
	}

	updated, cmd := m.Update(ToolResultMsg{
		Result: result,
		Call: services.ToolCall{
			Tool: "bash",
			Params: map[string]interface{}{
				"command": "echo hello",
			},
		},
	})
	if cmd != nil {
		t.Fatal("expected no follow-up command while waiting for approval")
	}

	got := updated.(Model)
	if got.chat.ToolExecuting {
		t.Fatal("expected tool executing flag to be cleared")
	}
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
