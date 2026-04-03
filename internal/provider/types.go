package provider

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ResponseID string     `json:"response_id,omitempty"`
	IsError    bool       `json:"is_error,omitempty"`
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}

type ChatRequest struct {
	Model        string     `json:"model"`
	SystemPrompt string     `json:"system_prompt"`
	Messages     []Message  `json:"messages"`
	Tools        []ToolSpec `json:"tools,omitempty"`
}

type ChatResponse struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
	Usage        Usage   `json:"usage"`
}

type Usage struct {
	InputTokens       int `json:"input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	TotalTokens       int `json:"total_tokens"`
	CachedInputTokens int `json:"cached_input_tokens,omitempty"`
	ReasoningTokens   int `json:"reasoning_tokens,omitempty"`
}

type ModelDescriptor struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	ContextWindow   int             `json:"context_window,omitempty"`
	MaxOutputTokens int             `json:"max_output_tokens,omitempty"`
	Capabilities    map[string]bool `json:"capabilities,omitempty"`
}

type ProviderCatalogItem struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Models      []ModelDescriptor `json:"models,omitempty"`
}

type ProviderSelection struct {
	ProviderID string `json:"provider_id"`
	ModelID    string `json:"model_id"`
}

type StreamEventType string

const (
	// StreamEventTextDelta reports streamed assistant text.
	StreamEventTextDelta StreamEventType = "text_delta"
	// StreamEventToolCallStart reports that the model has started a tool call.
	StreamEventToolCallStart StreamEventType = "tool_call_start"
	// StreamEventToolCallDelta reports incremental tool-call arguments.
	StreamEventToolCallDelta StreamEventType = "tool_call_delta"
	// StreamEventReasoningDelta reports incremental reasoning-summary text.
	StreamEventReasoningDelta StreamEventType = "reasoning_delta"
	// StreamEventMessageDone reports that the current assistant turn has finished.
	StreamEventMessageDone StreamEventType = "message_done"
)

// StreamEvent is emitted by a provider while a chat request is streaming.
type StreamEvent struct {
	Type StreamEventType

	// text_delta
	Text string `json:"text,omitempty"`

	// tool_call_start / tool_call_delta
	ToolCallIndex      int    `json:"tool_call_index,omitempty"`
	ToolCallID         string `json:"tool_call_id,omitempty"`
	ToolName           string `json:"tool_name,omitempty"`
	ToolArgumentsDelta string `json:"tool_arguments_delta,omitempty"`

	// reasoning_delta
	ReasoningText string `json:"reasoning_text,omitempty"`

	// message_done
	FinishReason string `json:"finish_reason,omitempty"`
	ResponseID   string `json:"response_id,omitempty"`
	Usage        *Usage `json:"usage,omitempty"`
}
