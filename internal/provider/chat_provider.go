package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"neo-code/internal/agentruntime/chat"
	"neo-code/internal/config"
)

const (
	requestTimeout = 90 * time.Second
	maxRetries     = 2
)

var (
	ErrInvalidAPIKey        = errors.New("invalid API key")
	ErrAPIKeyValidationSoft = errors.New("API key validation could not be completed")
)

type ChatCompletionProvider struct {
	APIKey  string
	BaseURL string
	Model   string
}

func (p *ChatCompletionProvider) GetModelName() string {
	if p.Model != "" {
		return p.Model
	}
	return DefaultModel()
}

type streamResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func NewChatProvider(model string) (chat.ChatProvider, error) {
	if config.GlobalAppConfig == nil {
		return nil, fmt.Errorf("app config is not loaded")
	}

	providerName := CurrentProvider()
	if model == "" {
		model = DefaultModel()
	}
	if model == "" {
		return nil, fmt.Errorf("ai.model is required for provider %s", providerName)
	}
	baseURL, err := ResolveChatEndpoint(config.GlobalAppConfig, model)
	if err != nil {
		return nil, err
	}
	apiKey := config.RuntimeAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("missing %s environment variable", config.RuntimeAPIKeyEnvVarName())
	}

	switch CurrentProviderProtocol(config.GlobalAppConfig) {
	case "anthropic":
		return &AnthropicProvider{
			APIKey:  apiKey,
			BaseURL: baseURL,
			Model:   model,
		}, nil
	case "gemini":
		return &GeminiProvider{
			APIKey:  apiKey,
			BaseURL: baseURL,
			Model:   model,
		}, nil
	default:
		return &ChatCompletionProvider{
			APIKey:  apiKey,
			BaseURL: baseURL,
			Model:   model,
		}, nil
	}
}

func ValidateChatAPIKey(ctx context.Context, cfg *config.AppConfiguration) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	providerName := providerNameFromConfig(cfg)
	if providerName == "" {
		return fmt.Errorf("unsupported provider: %s", cfg.AI.Provider)
	}
	if strings.TrimSpace(cfg.CurrentModelName()) == "" {
		cfg.SetCurrentModel(DefaultModelForProvider(providerName))
	}
	if strings.TrimSpace(cfg.CurrentModelName()) == "" {
		return fmt.Errorf("current model is required for provider %s", providerName)
	}

	return validateChatAPIKey(ctx, cfg)
}

func (p *ChatCompletionProvider) Chat(ctx context.Context, messages []chat.Message) (<-chan string, error) {
	baseURL := strings.TrimSpace(p.BaseURL)
	if baseURL == "" {
		var err error
		baseURL, err = ResolveChatEndpoint(config.GlobalAppConfig, p.GetModelName())
		if err != nil {
			return nil, err
		}
	}

	body := map[string]any{
		"model":    p.GetModelName(),
		"messages": messages,
		"stream":   true,
	}
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("chat request marshal failed: %w", err)
	}

	resp, err := doRequestWithRetry(ctx, func(reqCtx context.Context) (*http.Response, error) {
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, baseURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("chat request create failed: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient().Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("retryable chat status: %s %s", resp.Status, strings.TrimSpace(string(body)))
		}
		return resp, nil
	})
	if err != nil {
		return nil, fmt.Errorf("chat request failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat request failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}

	out := make(chan string)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				emitStreamErrorMessage(ctx, out, streamReadError(err))
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "" {
				continue
			}
			if data == "[DONE]" {
				return
			}

			text, err := decodeStreamContent(data)
			if err != nil {
				emitStreamErrorMessage(ctx, out, err)
				return
			}
			if text == "" {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case out <- text:
			}
		}
	}()

	return out, nil
}

func emitStreamErrorMessage(ctx context.Context, out chan<- string, err error) {
	if err == nil {
		return
	}
	msg := fmt.Sprintf("\n[STREAM_ERROR] %v", err)
	select {
	case <-ctx.Done():
	case out <- msg:
	}
}

func streamReadError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, io.EOF) {
		return fmt.Errorf("chat stream ended unexpectedly before completion")
	}
	return fmt.Errorf("chat stream read failed: %w", err)
}

func decodeStreamContent(data string) (string, error) {
	var res streamResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		return "", fmt.Errorf("chat stream decode failed: %w", err)
	}
	if len(res.Choices) == 0 {
		return "", nil
	}
	return stripThinkingTags(res.Choices[0].Delta.Content), nil
}

func stripThinkingTags(content string) string {
	const (
		thinkStart = "<think>"
		thinkEnd   = "</think>"
	)
	for {
		start := strings.Index(content, thinkStart)
		if start == -1 {
			break
		}
		end := strings.Index(content, thinkEnd)
		if end == -1 {
			break
		}
		content = content[:start] + content[end+len(thinkEnd):]
	}
	return content
}

func httpClient() *http.Client {
	return &http.Client{Timeout: requestTimeout}
}

func doRequestWithRetry(ctx context.Context, do func(context.Context) (*http.Response, error)) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := do(ctx)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if ctx.Err() != nil || !isRetryableError(err) || attempt == maxRetries {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 300 * time.Millisecond)
	}
	return nil, lastErr
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return strings.Contains(err.Error(), "retryable chat status:")
}

func validateChatAPIKey(ctx context.Context, cfg *config.AppConfiguration) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	modelName := strings.TrimSpace(cfg.CurrentModelName())
	if modelName == "" {
		modelName = DefaultModelForProvider(cfg.CurrentProviderName())
	}
	baseURL, err := ResolveChatEndpoint(cfg, modelName)
	if err != nil {
		return err
	}
	return validateByProtocol(ctx, cfg, modelName, baseURL, CurrentProviderProtocol(cfg))
}
