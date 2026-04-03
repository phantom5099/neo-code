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

func TestBuildRequestMapsConversationAndContinuation(t *testing.T) {
	t.Parallel()

	provider, err := New(resolvedConfig(config.OpenAIDefaultBaseURL, config.OpenAIDefaultModel))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	payload, err := provider.buildRequest(domain.ChatRequest{
		SystemPrompt: "system prompt",
		Messages: []domain.Message{
			{Role: domain.RoleUser, Content: "older message"},
			{Role: domain.RoleAssistant, Content: "prior answer", ResponseID: "resp_prev"},
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
	if decoded["instructions"] != "system prompt" {
		t.Fatalf("expected instructions to carry system prompt, got %#v", decoded["instructions"])
	}
	if decoded["previous_response_id"] != "resp_prev" {
		t.Fatalf("expected previous_response_id resp_prev, got %#v", decoded["previous_response_id"])
	}

	input, ok := decoded["input"].([]any)
	if !ok {
		t.Fatalf("expected input array, got %#v", decoded["input"])
	}
	if len(input) != 2 {
		t.Fatalf("expected 2 tail input items, got %d", len(input))
	}

	toolOutput, ok := input[0].(map[string]any)
	if !ok {
		t.Fatalf("expected function_call_output map, got %#v", input[0])
	}
	if toolOutput["type"] != "function_call_output" {
		t.Fatalf("expected function_call_output item, got %#v", toolOutput["type"])
	}
	if toolOutput["call_id"] != "call_err" {
		t.Fatalf("expected tool output call_id call_err, got %#v", toolOutput["call_id"])
	}

	outputText, ok := toolOutput["output"].(string)
	if !ok {
		t.Fatalf("expected string output payload, got %#v", toolOutput["output"])
	}
	if !strings.Contains(outputText, `"is_error":true`) || !strings.Contains(outputText, "permission denied") {
		t.Fatalf("expected structured tool error output, got %q", outputText)
	}

	tools, ok := decoded["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected one function tool, got %#v", decoded["tools"])
	}
	tool, ok := tools[0].(map[string]any)
	if !ok {
		t.Fatalf("expected tool payload map, got %#v", tools[0])
	}
	if tool["type"] != "function" || tool["name"] != "filesystem_edit" {
		t.Fatalf("unexpected tool payload: %#v", tool)
	}
}

func TestBuildRequestMapsAssistantToolCalls(t *testing.T) {
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
	input := decoded["input"].([]any)
	if len(input) != 2 {
		t.Fatalf("expected 2 input items, got %d", len(input))
	}
	functionCall := input[1].(map[string]any)
	if functionCall["type"] != "function_call" {
		t.Fatalf("expected function_call item, got %#v", functionCall["type"])
	}
	if functionCall["call_id"] != "call_1" || functionCall["name"] != "filesystem_edit" {
		t.Fatalf("unexpected function_call payload: %#v", functionCall)
	}
}

func TestProviderChatStreamsResponsesAPIEvents(t *testing.T) {
	t.Setenv(config.OpenAIDefaultAPIKeyEnv, "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["model"] != "gpt-5.4" {
			t.Fatalf("expected model gpt-5.4, got %#v", payload["model"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		writeSSEChunk(t, w, map[string]any{
			"type":            "response.output_item.added",
			"sequence_number": 1,
			"output_index":    0,
			"item": map[string]any{
				"type":      "function_call",
				"id":        "fc_1",
				"call_id":   "call_1",
				"name":      "filesystem_edit",
				"arguments": "",
				"status":    "in_progress",
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"type":            "response.function_call_arguments.delta",
			"sequence_number": 2,
			"output_index":    0,
			"item_id":         "fc_1",
			"delta":           `{"path":"main.go"}`,
		})
		writeSSEChunk(t, w, map[string]any{
			"type":            "response.output_text.delta",
			"sequence_number": 3,
			"output_index":    1,
			"content_index":   0,
			"item_id":         "msg_1",
			"delta":           "Hello ",
			"logprobs":        []any{},
		})
		writeSSEChunk(t, w, map[string]any{
			"type":            "response.output_text.delta",
			"sequence_number": 4,
			"output_index":    1,
			"content_index":   0,
			"item_id":         "msg_1",
			"delta":           "world",
			"logprobs":        []any{},
		})
		writeSSEChunk(t, w, map[string]any{
			"type":            "response.reasoning_summary_text.delta",
			"sequence_number": 5,
			"output_index":    2,
			"summary_index":   0,
			"item_id":         "rs_1",
			"delta":           "thinking",
		})
		writeSSEChunk(t, w, map[string]any{
			"type":            "response.completed",
			"sequence_number": 6,
			"response": map[string]any{
				"id": "resp_123",
				"output": []map[string]any{
					{
						"type":      "function_call",
						"id":        "fc_1",
						"call_id":   "call_1",
						"name":      "filesystem_edit",
						"arguments": `{"path":"main.go"}`,
						"status":    "completed",
					},
					{
						"type":   "message",
						"id":     "msg_1",
						"role":   "assistant",
						"status": "completed",
						"content": []map[string]any{
							{
								"type":        "output_text",
								"text":        "Hello world",
								"annotations": []any{},
							},
						},
					},
				},
				"usage": map[string]any{
					"input_tokens":  10,
					"output_tokens": 5,
					"total_tokens":  15,
					"input_tokens_details": map[string]any{
						"cached_tokens": 2,
					},
					"output_tokens_details": map[string]any{
						"reasoning_tokens": 1,
					},
				},
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
				Schema: map[string]any{
					"type": "object",
				},
			},
		},
	}, events)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if response.Message.Content != "Hello world" {
		t.Fatalf("expected content %q, got %q", "Hello world", response.Message.Content)
	}
	if response.Message.ResponseID != "resp_123" {
		t.Fatalf("expected response id resp_123, got %q", response.Message.ResponseID)
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

	close(events)

	var (
		text             strings.Builder
		toolCallStart    *domain.StreamEvent
		toolCallDelta    *domain.StreamEvent
		reasoningDelta   *domain.StreamEvent
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
		case domain.StreamEventReasoningDelta:
			copied := event
			reasoningDelta = &copied
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
	if reasoningDelta == nil || reasoningDelta.ReasoningText != "thinking" {
		t.Fatalf("unexpected reasoning_delta event: %+v", reasoningDelta)
	}
	if messageDoneEvent == nil {
		t.Fatal("expected message_done event")
	}
	if messageDoneEvent.ResponseID != "resp_123" || messageDoneEvent.FinishReason != "tool_calls" {
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

func TestProviderChatResponseFailedEvent(t *testing.T) {
	t.Setenv(config.OpenAIDefaultAPIKeyEnv, "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSEChunk(t, w, map[string]any{
			"type":            "response.failed",
			"sequence_number": 1,
			"response": map[string]any{
				"id": "resp_failed",
				"error": map[string]any{
					"code":    "rate_limit_exceeded",
					"message": "slow down",
				},
			},
		})
	}))
	defer server.Close()

	provider, err := New(resolvedConfig(server.URL, config.OpenAIDefaultModel), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = provider.Chat(context.Background(), domain.ChatRequest{
		Model: config.OpenAIDefaultModel,
		Messages: []domain.Message{
			{Role: domain.RoleUser, Content: "hello"},
		},
	}, make(chan domain.StreamEvent, 1))
	if err == nil {
		t.Fatal("expected error")
	}

	var providerErr *domain.ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerErr.Code != domain.ErrorCodeRateLimit {
		t.Fatalf("expected rate limit error, got %s", providerErr.Code)
	}
	if !providerErr.Retryable {
		t.Fatalf("expected failed response rate limit error to be retryable")
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
