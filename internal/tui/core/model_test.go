package core

import (
	"context"
	"testing"

	"neo-code/internal/config"
	"neo-code/internal/tui/services"
	"neo-code/internal/tui/state"
)

func TestNewModelAppliesDefaultsAndRuntimeFlags(t *testing.T) {
	restoreCoreGlobals(t)

	client := &fakeChatClient{defaultModelName: "demo-model"}
	t.Setenv(config.DefaultAPIKeyEnvVar, "secret")
	config.GlobalAppConfig = nil

	m := NewModel(services.NewRuntimeController(client, "config.yaml"), "D:/neo-code")
	m.applyBootstrap(m.controller.Bootstrap(context.Background()))

	if m.chat.ActiveModel != "demo-model" {
		t.Fatalf("expected default model from client, got %q", m.chat.ActiveModel)
	}
	if !m.chat.APIKeyReady {
		t.Fatal("expected API key readiness to reflect runtime env var")
	}
	if m.chat.WorkspaceRoot != "D:/neo-code" {
		t.Fatalf("expected workspace root to be stored, got %q", m.chat.WorkspaceRoot)
	}
}

func TestNewModelUsesEmptyStatsWhenClientReturnsNil(t *testing.T) {
	restoreCoreGlobals(t)

	client := &fakeChatClient{nilMemoryStats: true}

	m := NewModel(services.NewRuntimeController(client, "config.yaml"), "D:/neo-code")
	m.applyBootstrap(m.controller.Bootstrap(context.Background()))
	if m.chat.MemoryStats.TotalItems != 0 {
		t.Fatalf("expected zero-value stats, got %+v", m.chat.MemoryStats)
	}
}

func TestAppendAndFinishLastMessage(t *testing.T) {
	m := Model{}
	m.chat.Messages = []state.Message{{Role: "assistant", Content: "hello", Streaming: true}}

	m.AppendLastMessage(" world")
	m.FinishLastMessage()

	if m.chat.Messages[0].Content != "hello world" {
		t.Fatalf("expected appended content, got %q", m.chat.Messages[0].Content)
	}
	if m.chat.Messages[0].Streaming {
		t.Fatal("expected last message streaming to be cleared")
	}
}

func TestInitReturnsNonNilCmd(t *testing.T) {
	restoreCoreGlobals(t)

	m := NewModel(services.NewRuntimeController(&fakeChatClient{}, "config.yaml"), "D:/neo-code")
	if cmd := m.Init(); cmd == nil {
		t.Fatal("expected non-nil init cmd")
	}
}

func TestNewModelAddsResumeSummaryMessageWhenSupported(t *testing.T) {
	restoreCoreGlobals(t)

	client := &resumeSummaryClient{
		fakeChatClient: fakeChatClient{defaultModelName: "demo-model"},
		summary:        "Recovered previous working context:\n- Current goal: fix the memory module",
	}

	m := NewModel(services.NewRuntimeController(client, "config.yaml"), "D:/neo-code")
	m.applyBootstrap(m.controller.Bootstrap(context.Background()))
	if len(m.chat.Messages) != 1 {
		t.Fatalf("expected one resume summary message, got %+v", m.chat.Messages)
	}
	if m.chat.Messages[0].Role != "system" || m.chat.Messages[0].Kind != services.MessageKindResumeSummary {
		t.Fatalf("expected resume summary system message, got %+v", m.chat.Messages[0])
	}
}

func TestBuildRequestMessagesKeepsSystemMessagesAndLatestTurns(t *testing.T) {
	restoreCoreGlobals(t)
	config.GlobalAppConfig = config.DefaultAppConfig()
	config.GlobalAppConfig.History.ShortTermTurns = 2

	got := services.BuildRequestMessages([]services.SessionMessage{
		{Role: "system", Content: "persona"},
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "u2"},
		{Role: "assistant", Content: "a2"},
		{Role: "user", Content: "u3"},
		{Role: "assistant", Content: "a3"},
	})

	if len(got) != 5 {
		t.Fatalf("expected system message plus last two turns, got %d messages", len(got))
	}
	if got[0].Role != "system" || got[0].Content != "persona" {
		t.Fatalf("expected system message to be preserved, got %+v", got[0])
	}
	if got[1].Content != "u2" || got[4].Content != "a3" {
		t.Fatalf("expected only latest turns to remain, got %+v", got)
	}
}

type resumeSummaryClient struct {
	fakeChatClient
	summary string
}

func (c *resumeSummaryClient) GetWorkingSessionSummary(context.Context) (string, error) {
	return c.summary, nil
}
