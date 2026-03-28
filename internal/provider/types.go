package provider

import (
	"errors"
	"strings"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
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
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

var (
	ErrProviderNotFound = errors.New("provider not found")
	ErrModelNotFound    = errors.New("model not found")
	ErrDriverNotFound   = errors.New("provider driver not found")
)

type ModelDescriptor struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type ProviderCatalogItem struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	APIKeyEnv   string            `json:"api_key_env,omitempty"`
	Models      []ModelDescriptor `json:"models,omitempty"`
}

type ProviderSelection struct {
	ProviderID string `json:"provider_id"`
	ModelID    string `json:"model_id"`
}

func normalizeCatalogItem(item ProviderCatalogItem, fallbackID string, fallbackEnv string) ProviderCatalogItem {
	if strings.TrimSpace(item.ID) == "" {
		item.ID = strings.TrimSpace(fallbackID)
	}
	if strings.TrimSpace(item.Name) == "" {
		item.Name = item.ID
	}
	if strings.TrimSpace(item.APIKeyEnv) == "" {
		item.APIKeyEnv = strings.TrimSpace(fallbackEnv)
	}

	item.Models = normalizeModels(item.Models)
	return item
}

func normalizeModels(models []ModelDescriptor) []ModelDescriptor {
	if len(models) == 0 {
		return nil
	}

	deduped := make([]ModelDescriptor, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		id := strings.TrimSpace(model.ID)
		name := strings.TrimSpace(model.Name)
		if id == "" {
			id = name
		}
		if id == "" {
			continue
		}
		if name == "" {
			name = id
		}
		key := strings.ToLower(id)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, ModelDescriptor{
			ID:          id,
			Name:        name,
			Description: strings.TrimSpace(model.Description),
		})
	}

	return deduped
}
