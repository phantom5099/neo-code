package openai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"neo-code/internal/config"
	domain "neo-code/internal/provider"
)

func TestDriver(t *testing.T) {
	t.Parallel()

	driver := Driver()
	if driver.Name != DriverName {
		t.Fatalf("expected driver name %q, got %q", DriverName, driver.Name)
	}
	if driver.Build == nil {
		t.Fatal("expected Build function to be non-nil")
	}
	if driver.Discover == nil {
		t.Fatal("expected Discover function to be non-nil")
	}
}

func TestWithTransport(t *testing.T) {
	t.Parallel()

	customTransport := &http.Transport{}
	provider, err := New(resolvedConfig("", ""), WithTransport(customTransport))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if provider.httpClient == nil {
		t.Fatal("expected custom transport to force a dedicated HTTP client")
	}
	if provider.httpClient.Transport != customTransport {
		t.Fatal("expected custom transport to be set on HTTP client")
	}
}

func TestDiscoverModels(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":                "gpt-5.4",
					"name":              "GPT-5.4",
					"context_window":    128000,
					"max_output_tokens": 8192,
				},
				{"id": "gpt-4.1"},
			},
		})
	}))
	defer server.Close()

	provider, err := New(resolvedConfig(server.URL, ""), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	models, err := provider.DiscoverModels(context.Background())
	if err != nil {
		t.Fatalf("DiscoverModels() error = %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].ID != "gpt-5.4" || models[0].Name != "GPT-5.4" {
		t.Fatalf("unexpected first model: %+v", models[0])
	}
	if models[0].ContextWindow != 128000 || models[0].MaxOutputTokens != 8192 {
		t.Fatalf("expected model metadata to be preserved, got %+v", models[0])
	}
}

func TestBuildRequestMapsMessagesForChatCompletions(t *testing.T) {
	t.Parallel()

	provider, err := New(resolvedConfig(config.OpenAIDefaultBaseURL, config.OpenAIDefaultModel))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	payload, err := provider.buildRequest(domain.ChatRequest{
		SystemPrompt: "You are a coding assistant.",
		Messages: []domain.Message{
			{Role: domain.RoleUser, Content: "older message"},
			{Role: domain.RoleAssistant, Content: "prior answer"},
			{Role: domain.RoleTool, ToolCallID: "call_err", Content: "permission denied", IsError: true},
			{Role: domain.RoleUser, Content: "please try again"},
		},
		Tools: []domain.ToolSpec{
			{
				Name:        "filesystem_edit",
				Description: "Edit file content",
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildRequest() error = %v", err)
	}

	decoded := mustMarshalAndDecode(t, payload)

	if decoded["model"] != config.OpenAIDefaultModel {
		t.Fatalf("expected default model %q, got %#v", config.OpenAIDefaultModel, decoded["model"])
	}

	messages, ok := decoded["messages"].([]any)
	if !ok {
		t.Fatalf("expected messages array, got %#v", decoded["messages"])
	}

	// messages[0] should be system prompt
	sysMsg, _ := messages[0].(map[string]any)
	if sysMsg["role"] != "system" || sysMsg["content"] != "You are a coding assistant." {
		t.Fatalf("expected system message first, got %#v", sysMsg)
	}

	// Find the tool message (should have structured error output)
	foundToolErr := false
	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if msg["role"] == "tool" {
			content, _ := msg["content"].(string)
			if strings.Contains(content, `"is_error":true`) && strings.Contains(content, "permission denied") {
				foundToolErr = true
			}
			break
		}
	}
	if !foundToolErr {
		t.Fatal("expected tool message with structured error output")
	}

	tools, ok := decoded["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected one function tool, got %#v", decoded["tools"])
	}
	tool, ok := tools[0].(map[string]any)
	if !ok {
		t.Fatalf("expected tool payload map, got %#v", tools[0])
	}
	if tool["type"] != "function" || tool["function"].(map[string]any)["name"] != "filesystem_edit" {
		t.Fatalf("unexpected tool payload: %#v", tool)
	}
}

func TestBuildRequestMapsAssistantToolCallsForChatCompletions(t *testing.T) {
	t.Parallel()

	provider, err := New(resolvedConfig(config.OpenAIDefaultBaseURL, config.OpenAIDefaultModel))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	payload, err := provider.buildRequest(domain.ChatRequest{
		Messages: []domain.Message{
			{Role: domain.RoleUser, Content: "edit main.go"},
			{
				Role: domain.RoleAssistant,
				ToolCalls: []domain.ToolCall{
					{
						ID:        "call_1",
						Name:      "filesystem_edit",
						Arguments: `{"path":"main.go"}`,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildRequest() error = %v", err)
	}

	decoded := mustMarshalAndDecode(t, payload)
	messages := decoded["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	assistantMsg := messages[1].(map[string]any)
	if assistantMsg["role"] != "assistant" {
		t.Fatalf("expected assistant role, got %#v", assistantMsg["role"])
	}

	toolCalls, ok := assistantMsg["tool_calls"].([]any)
	if !ok || len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool_call in assistant message, got %#v", assistantMsg)
	}
	tc := toolCalls[0].(map[string]any)
	fn := tc["function"].(map[string]any)
	if tc["id"] != "call_1" || fn["name"] != "filesystem_edit" || fn["arguments"] != `{"path":"main.go"}` {
		t.Fatalf("unexpected tool_call payload: %#v", tc)
	}
}

func TestProviderChatStreamsChatCompletionsEvents(t *testing.T) {
	t.Setenv(config.OpenAIDefaultAPIKeyEnv, "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s, expected /chat/completions", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["model"] != "gpt-5.4" {
			t.Fatalf("expected model gpt-5.4, got %#v", payload["model"])
		}

		w.Header().Set("Content-Type", "text/event-stream")

		// Chunk 1: role + tool_call start
		writeSSEChunk(t, w, map[string]any{
			"id": "chatcmpl-xxx",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{
							{
								"index":    0,
								"id":       "call_1",
								"type":     "function",
								"function": map[string]any{"name": "filesystem_edit", "arguments": ""},
							},
						},
					},
					"finish_reason": nil,
				},
			},
		})
		// Chunk 2: tool_call arguments delta
		writeSSEChunk(t, w, map[string]any{
			"id": "chatcmpl-xxx",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"tool_calls": []map[string]any{
							{
								"index":    0,
								"function": map[string]any{"arguments": `{"path":"main.go"}`},
							},
						},
					},
					"finish_reason": nil,
				},
			},
		})
		// Chunk 3: text delta
		writeSSEChunk(t, w, map[string]any{
			"id": "chatcmpl-xxx",
			"choices": []map[string]any{
				{
					"index":         0,
					"delta":         map[string]any{"content": "Hello "},
					"finish_reason": nil,
				},
			},
		})
		// Chunk 4: text delta continued
		writeSSEChunk(t, w, map[string]any{
			"id": "chatcmpl-xxx",
			"choices": []map[string]any{
				{
					"index":         0,
					"delta":         map[string]any{"content": "world"},
					"finish_reason": nil,
				},
			},
		})
		// Final chunk: finish_reason + usage
		writeSSEChunk(t, w, map[string]any{
			"id": "chatcmpl-xxx",
			"choices": []map[string]any{
				{
					"index":         0,
					"delta":         map[string]any{},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":             10,
				"completion_tokens":         5,
				"total_tokens":              15,
				"prompt_tokens_details":     map[string]any{"cached_tokens": 2},
				"completion_tokens_details": map[string]any{"reasoning_tokens": 1},
			},
		})
	}))
	defer server.Close()

	provider, err := New(resolvedConfig(server.URL, "gpt-5.4"), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	events := make(chan domain.StreamEvent, 16)
	response, err := provider.Chat(context.Background(), domain.ChatRequest{
		Model: "gpt-5.4",
		Messages: []domain.Message{
			{Role: domain.RoleUser, Content: "please edit the file"},
		},
		Tools: []domain.ToolSpec{
			{
				Name:        "filesystem_edit",
				Description: "Edit one matching block in a file",
				Schema:      map[string]any{"type": "object"},
			},
		},
	}, events)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if response.Message.Content != "Hello world" {
		t.Fatalf("expected content %q, got %q", "Hello world", response.Message.Content)
	}
	if response.FinishReason != "tool_calls" {
		t.Fatalf("expected finish reason tool_calls, got %q", response.FinishReason)
	}
	if response.Usage.TotalTokens != 15 || response.Usage.CachedInputTokens != 2 || response.Usage.ReasoningTokens != 1 {
		t.Fatalf("unexpected usage: %+v", response.Usage)
	}
	if len(response.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(response.Message.ToolCalls))
	}
	if response.Message.ToolCalls[0].ID != "call_1" || response.Message.ToolCalls[0].Name != "filesystem_edit" {
		t.Fatalf("unexpected tool call: %+v", response.Message.ToolCalls[0])
	}

	close(events)

	var (
		text             strings.Builder
		toolCallStart    *domain.StreamEvent
		toolCallDelta    *domain.StreamEvent
		messageDoneEvent *domain.StreamEvent
	)
	for event := range events {
		switch event.Type {
		case domain.StreamEventTextDelta:
			text.WriteString(event.Text)
		case domain.StreamEventToolCallStart:
			copied := event
			toolCallStart = &copied
		case domain.StreamEventToolCallDelta:
			copied := event
			toolCallDelta = &copied
		case domain.StreamEventMessageDone:
			copied := event
			messageDoneEvent = &copied
		}
	}

	if text.String() != "Hello world" {
		t.Fatalf("expected streamed chunks to form %q, got %q", "Hello world", text.String())
	}
	if toolCallStart == nil || toolCallStart.ToolCallID != "call_1" || toolCallStart.ToolName != "filesystem_edit" {
		t.Fatalf("unexpected tool_call_start event: %+v", toolCallStart)
	}
	if toolCallDelta == nil || toolCallDelta.ToolArgumentsDelta != `{"path":"main.go"}` {
		t.Fatalf("unexpected tool_call_delta event: %+v", toolCallDelta)
	}
	if messageDoneEvent == nil {
		t.Fatal("expected message_done event")
	}
	if messageDoneEvent.FinishReason != "tool_calls" {
		t.Fatalf("unexpected message_done event: %+v", messageDoneEvent)
	}
	if messageDoneEvent.Usage == nil || messageDoneEvent.Usage.CachedInputTokens != 2 || messageDoneEvent.Usage.ReasoningTokens != 1 {
		t.Fatalf("unexpected message_done usage: %+v", messageDoneEvent.Usage)
	}
}

func TestProviderChatHTTPErrorResponses(t *testing.T) {
	t.Setenv(config.OpenAIDefaultAPIKeyEnv, "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer server.Close()

	provider, err := New(resolvedConfig(server.URL, config.OpenAIDefaultModel), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = provider.Chat(context.Background(), domain.ChatRequest{
		Model: config.OpenAIDefaultModel,
	}, make(chan domain.StreamEvent, 1))
	if err == nil {
		t.Fatal("expected error")
	}

	var providerErr *domain.ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerErr.Code != domain.ErrorCodeAuthFailed {
		t.Fatalf("expected auth_failed, got %s", providerErr.Code)
	}
	if !strings.Contains(providerErr.Message, "invalid api key") {
		t.Fatalf("expected error message to include invalid api key, got %q", providerErr.Message)
	}
}

func resolvedConfig(baseURL string, model string) config.ResolvedProviderConfig {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = config.OpenAIDefaultBaseURL
	}
	if strings.TrimSpace(model) == "" {
		model = config.OpenAIDefaultModel
	}

	return config.ResolvedProviderConfig{
		ProviderConfig: config.ProviderConfig{
			Name:      DriverName,
			Driver:    DriverName,
			BaseURL:   baseURL,
			Model:     model,
			APIKeyEnv: config.OpenAIDefaultAPIKeyEnv,
		},
		APIKey: "test-key",
	}
}

func mustMarshalAndDecode(t *testing.T, value any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return decoded
}

func writeSSEChunk(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal SSE payload: %v", err)
	}
	if _, err := w.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
		t.Fatalf("write SSE payload: %v", err)
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
