package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"neo-code/internal/agentruntime/chat"
	"neo-code/internal/config"
)

const anthropicVersion = "2023-06-01"

type AnthropicProvider struct {
	APIKey  string
	BaseURL string
	Model   string
}

func (p *AnthropicProvider) GetModelName() string {
	return strings.TrimSpace(p.Model)
}

func (p *AnthropicProvider) Chat(ctx context.Context, messages []chat.Message) (<-chan string, error) {
	body, err := buildAnthropicBody(p.GetModelName(), messages, true)
	if err != nil {
		return nil, err
	}
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("chat request marshal failed: %w", err)
	}

	resp, err := doRequestWithRetry(ctx, func(reqCtx context.Context) (*http.Response, error) {
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, p.BaseURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("chat request create failed: %w", err)
		}
		req.Header.Set("x-api-key", p.APIKey)
		req.Header.Set("anthropic-version", anthropicVersion)
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient().Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
			payload, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("retryable chat status: %s %s", resp.Status, strings.TrimSpace(string(payload)))
		}
		return resp, nil
	})
	if err != nil {
		return nil, fmt.Errorf("chat request failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat request failed: %s %s", resp.Status, strings.TrimSpace(string(payload)))
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
			if line == "" || strings.HasPrefix(line, "event:") {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" || data == "[DONE]" {
				continue
			}

			text, stop, decodeErr := decodeAnthropicStream(data)
			if decodeErr != nil {
				emitStreamErrorMessage(ctx, out, decodeErr)
				return
			}
			if stop {
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

type GeminiProvider struct {
	APIKey  string
	BaseURL string
	Model   string
}

func (p *GeminiProvider) GetModelName() string {
	return strings.TrimSpace(p.Model)
}

func (p *GeminiProvider) Chat(ctx context.Context, messages []chat.Message) (<-chan string, error) {
	body, err := buildGeminiBody(messages)
	if err != nil {
		return nil, err
	}
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("chat request marshal failed: %w", err)
	}

	endpoint, err := appendAPIKeyQuery(strings.ReplaceAll(p.BaseURL, "{model}", p.GetModelName()), p.APIKey)
	if err != nil {
		return nil, err
	}

	resp, err := doRequestWithRetry(ctx, func(reqCtx context.Context) (*http.Response, error) {
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("chat request create failed: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient().Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
			payload, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("retryable chat status: %s %s", resp.Status, strings.TrimSpace(string(payload)))
		}
		return resp, nil
	})
	if err != nil {
		return nil, fmt.Errorf("chat request failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat request failed: %s %s", resp.Status, strings.TrimSpace(string(payload)))
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
			if line == "" || strings.HasPrefix(line, "event:") {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" || data == "[DONE]" {
				continue
			}
			text, decodeErr := decodeGeminiStream(data)
			if decodeErr != nil {
				emitStreamErrorMessage(ctx, out, decodeErr)
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

func validateByProtocol(ctx context.Context, cfg *config.AppConfiguration, modelName, baseURL, protocol string) error {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "anthropic":
		body, err := buildAnthropicBody(modelName, []chat.Message{{Role: "user", Content: "ping"}}, false)
		if err != nil {
			return err
		}
		return validateJSONEndpoint(ctx, baseURL, body, func(req *http.Request) {
			req.Header.Set("x-api-key", cfg.RuntimeAPIKey())
			req.Header.Set("anthropic-version", anthropicVersion)
			req.Header.Set("Content-Type", "application/json")
		})
	case "gemini":
		body, err := buildGeminiBody([]chat.Message{{Role: "user", Content: "ping"}})
		if err != nil {
			return err
		}
		validationURL, err := appendAPIKeyQuery(geminiValidationURL(baseURL, modelName), cfg.RuntimeAPIKey())
		if err != nil {
			return err
		}
		return validateJSONEndpoint(ctx, validationURL, body, func(req *http.Request) {
			req.Header.Set("Content-Type", "application/json")
		})
	default:
		body := map[string]any{
			"model":    modelName,
			"messages": []chat.Message{{Role: "user", Content: "ping"}},
			"stream":   false,
		}
		return validateJSONEndpoint(ctx, baseURL, body, func(req *http.Request) {
			req.Header.Set("Authorization", "Bearer "+cfg.RuntimeAPIKey())
			req.Header.Set("Content-Type", "application/json")
		})
	}
}

func validateJSONEndpoint(ctx context.Context, endpoint string, body any, prepare func(*http.Request)) error {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("validation request marshal failed: %w", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("validation request create failed: %w", err)
	}
	if prepare != nil {
		prepare(req)
	}

	resp, err := httpClient().Do(req)
	if err != nil {
		if requestCtx.Err() != nil || isRetryableError(err) {
			return fmt.Errorf("%w: %v", ErrAPIKeyValidationSoft, err)
		}
		return fmt.Errorf("validation failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("%w: %v", ErrAPIKeyValidationSoft, readErr)
	}

	switch {
	case resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices:
		return nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%w: %s", ErrInvalidAPIKey, strings.TrimSpace(string(bodyBytes)))
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError:
		return fmt.Errorf("%w: %s %s", ErrAPIKeyValidationSoft, resp.Status, strings.TrimSpace(string(bodyBytes)))
	default:
		return fmt.Errorf("validation failed: %s %s", resp.Status, strings.TrimSpace(string(bodyBytes)))
	}
}

func buildAnthropicBody(model string, messages []chat.Message, stream bool) (map[string]any, error) {
	system, conversational := splitSystemMessages(messages)
	anthropicMessages := make([]map[string]string, 0, len(conversational))
	for _, msg := range conversational {
		anthropicMessages = append(anthropicMessages, map[string]string{
			"role":    strings.TrimSpace(msg.Role),
			"content": msg.Content,
		})
	}
	body := map[string]any{
		"model":      model,
		"max_tokens": 4096,
		"messages":   anthropicMessages,
		"stream":     stream,
	}
	if system != "" {
		body["system"] = system
	}
	return body, nil
}

func buildGeminiBody(messages []chat.Message) (map[string]any, error) {
	system, conversational := splitSystemMessages(messages)
	contents := make([]map[string]any, 0, len(conversational))
	for _, msg := range conversational {
		role := "user"
		if strings.TrimSpace(msg.Role) == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]any{
			"role": role,
			"parts": []map[string]any{
				{"text": msg.Content},
			},
		})
	}

	body := map[string]any{"contents": contents}
	if system != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": system}},
		}
	}
	return body, nil
}

func splitSystemMessages(messages []chat.Message) (string, []chat.Message) {
	var systemParts []string
	converted := make([]chat.Message, 0, len(messages))
	for _, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		switch role {
		case "system":
			if strings.TrimSpace(msg.Content) != "" {
				systemParts = append(systemParts, msg.Content)
			}
		case "assistant", "user":
			converted = append(converted, chat.Message{Role: role, Content: msg.Content})
		}
	}
	if len(converted) == 0 {
		converted = append(converted, chat.Message{Role: "user", Content: "ping"})
	}
	return strings.Join(systemParts, "\n\n"), converted
}

func decodeAnthropicStream(data string) (string, bool, error) {
	var payload struct {
		Type  string `json:"type"`
		Delta struct {
			Text string `json:"text"`
		} `json:"delta"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return "", false, fmt.Errorf("anthropic stream decode failed: %w", err)
	}
	switch payload.Type {
	case "content_block_delta":
		return payload.Delta.Text, false, nil
	case "message_stop":
		return "", true, nil
	case "error":
		return "", false, fmt.Errorf("anthropic stream error: %s", strings.TrimSpace(payload.Error.Message))
	default:
		return "", false, nil
	}
}

func decodeGeminiStream(data string) (string, error) {
	var payload struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return "", fmt.Errorf("gemini stream decode failed: %w", err)
	}
	if strings.TrimSpace(payload.Error.Message) != "" {
		return "", fmt.Errorf("gemini stream error: %s", strings.TrimSpace(payload.Error.Message))
	}
	if len(payload.Candidates) == 0 || len(payload.Candidates[0].Content.Parts) == 0 {
		return "", nil
	}
	return payload.Candidates[0].Content.Parts[0].Text, nil
}

func geminiValidationURL(baseURL, model string) string {
	url := strings.ReplaceAll(baseURL, "{model}", model)
	url = strings.ReplaceAll(url, ":streamGenerateContent", ":generateContent")
	url = strings.ReplaceAll(url, "alt=sse", "")
	url = strings.TrimSuffix(url, "?")
	url = strings.TrimSuffix(url, "&")
	return url
}

func appendAPIKeyQuery(baseURL, apiKey string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse url %q: %w", baseURL, err)
	}
	query := parsed.Query()
	query.Set("key", apiKey)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
