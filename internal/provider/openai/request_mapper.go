package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared"

	"neo-code/internal/config"
	domain "neo-code/internal/provider"
)

// buildRequest maps a domain.ChatRequest to the OpenAI Chat Completions API request.
func (p *Provider) buildRequest(req domain.ChatRequest) (sdk.ChatCompletionNewParams, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(p.cfg.Model)
	}
	if model == "" {
		return sdk.ChatCompletionNewParams{}, errors.New("openai provider: model is empty")
	}

	params := sdk.ChatCompletionNewParams{
		Model: shared.ChatModel(model),
	}

	messages, err := mapMessagesToCC(req.SystemPrompt, req.Messages)
	if err != nil {
		return sdk.ChatCompletionNewParams{}, err
	}
	if len(messages) > 0 {
		params.Messages = messages
	}

	if tools, err := mapTools(req.Tools); err != nil {
		return sdk.ChatCompletionNewParams{}, err
	} else if len(tools) > 0 {
		params.Tools = tools
	}

	params.StreamOptions = sdk.ChatCompletionStreamOptionsParam{
		IncludeUsage: param.NewOpt(true),
	}

	return params, nil
}

// mapMessagesToCC builds the Chat Completions messages slice from a system prompt
// and conversation history.
func mapMessagesToCC(systemPrompt string, messages []domain.Message) ([]sdk.ChatCompletionMessageParamUnion, error) {
	var ccMessages []sdk.ChatCompletionMessageParamUnion

	if strings.TrimSpace(systemPrompt) != "" {
		ccMessages = append(ccMessages,
			sdk.ChatCompletionMessageParamUnion{
				OfSystem: &sdk.ChatCompletionSystemMessageParam{
					Content: sdk.ChatCompletionSystemMessageParamContentUnion{
						OfString: param.NewOpt(systemPrompt),
					},
				},
			},
		)
	}

	for i, msg := range messages {
		mapped, err := mapMessageToCC(msg)
		if err != nil {
			return nil, fmt.Errorf("openai provider: map message[%d]: %w", i, err)
		}
		ccMessages = append(ccMessages, mapped...)
	}
	return ccMessages, nil
}

// mapMessageToCC converts a single domain.Message into one or more Chat Completions message params.
func mapMessageToCC(message domain.Message) ([]sdk.ChatCompletionMessageParamUnion, error) {
	role := strings.TrimSpace(message.Role)
	switch role {
	case domain.RoleUser:
		content := strings.TrimSpace(message.Content)
		if content == "" {
			return nil, nil
		}
		return []sdk.ChatCompletionMessageParamUnion{
			{
				OfUser: &sdk.ChatCompletionUserMessageParam{
					Content: sdk.ChatCompletionUserMessageParamContentUnion{
						OfString: param.NewOpt(content),
					},
				},
			},
		}, nil

	case domain.RoleSystem:
		content := strings.TrimSpace(message.Content)
		if content == "" {
			return nil, nil
		}
		return []sdk.ChatCompletionMessageParamUnion{
			{
				OfSystem: &sdk.ChatCompletionSystemMessageParam{
					Content: sdk.ChatCompletionSystemMessageParamContentUnion{
						OfString: param.NewOpt(content),
					},
				},
			},
		}, nil

	case domain.RoleAssistant:
		asstMsg := sdk.ChatCompletionAssistantMessageParam{}
		if content := strings.TrimSpace(message.Content); content != "" {
			asstMsg.Content = sdk.ChatCompletionAssistantMessageParamContentUnion{
				OfString: param.NewOpt(content),
			}
		}
		for _, call := range message.ToolCalls {
			callID := strings.TrimSpace(call.ID)
			name := strings.TrimSpace(call.Name)
			args := strings.TrimSpace(call.Arguments)
			if callID == "" || name == "" {
				continue
			}
			asstMsg.ToolCalls = append(asstMsg.ToolCalls,
				sdk.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &sdk.ChatCompletionMessageFunctionToolCallParam{
						ID:   callID,
						Type: "function",
						Function: sdk.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name:      name,
							Arguments: args,
						},
					},
				},
			)
		}
		return []sdk.ChatCompletionMessageParamUnion{
			{OfAssistant: &asstMsg},
		}, nil

	case domain.RoleTool:
		callID := strings.TrimSpace(message.ToolCallID)
		if callID == "" {
			return nil, errors.New("tool message: tool_call_id is empty")
		}
		content, err := encodeToolOutput(message)
		if err != nil {
			return nil, err
		}
		return []sdk.ChatCompletionMessageParamUnion{
			{
				OfTool: &sdk.ChatCompletionToolMessageParam{
					Role:       domain.RoleTool,
					ToolCallID: callID,
					Content: sdk.ChatCompletionToolMessageParamContentUnion{
						OfString: param.NewOpt(content),
					},
				},
			},
		}, nil
	}

	return nil, fmt.Errorf("unsupported message role %q", role)
}

// mapTools converts domain tool specs into OpenAI function-calling tool definitions.
func mapTools(specs []domain.ToolSpec) ([]sdk.ChatCompletionToolUnionParam, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	tools := make([]sdk.ChatCompletionToolUnionParam, 0, len(specs))
	for i, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			return nil, fmt.Errorf("tool[%d]: name is empty", i)
		}
		fn := shared.FunctionDefinitionParam{
			Name:       name,
			Parameters: spec.Schema,
		}
		if desc := strings.TrimSpace(spec.Description); desc != "" {
			fn.Description = param.NewOpt(desc)
		}
		tools = append(tools, sdk.ChatCompletionToolUnionParam{
			OfFunction: &sdk.ChatCompletionFunctionToolParam{
				Type:     "function",
				Function: fn,
			},
		})
	}
	return tools, nil
}

// encodeToolOutput serializes a tool-result message's content for the API.
// When IsError is set, wraps the payload in the structured error format expected
// by the OpenAI function-calling convention (used by both Responses and CC APIs).
func encodeToolOutput(message domain.Message) (string, error) {
	if !message.IsError {
		return message.Content, nil
	}
	payload := struct {
		IsError bool   `json:"is_error"`
		Content string `json:"content,omitempty"`
	}{
		IsError: true,
		Content: message.Content,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("openai provider: marshal tool error output: %w", err)
	}
	return string(data), nil
}

// descriptorFromSDKModel extracts model metadata from an SDK Model object.
func descriptorFromSDKModel(model sdk.Model) (config.ModelDescriptor, bool) {
	rawJSON := strings.TrimSpace(model.RawJSON())
	if rawJSON == "" {
		return config.ModelDescriptor{}, false
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		return config.ModelDescriptor{}, false
	}

	return config.DescriptorFromRawModel(raw)
}
