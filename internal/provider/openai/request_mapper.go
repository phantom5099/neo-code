package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	domain "neo-code/internal/provider"
)

func (p *Provider) buildRequest(req domain.ChatRequest) (responses.ResponseNewParams, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(p.cfg.Model)
	}
	if model == "" {
		return responses.ResponseNewParams{}, errors.New("openai provider: model is empty")
	}

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(model),
	}

	if instructions := strings.TrimSpace(req.SystemPrompt); instructions != "" {
		params.Instructions = param.NewOpt(instructions)
	}

	previousResponseID, tail := splitMessagesForContinuation(req.Messages)
	if previousResponseID != "" {
		params.PreviousResponseID = param.NewOpt(previousResponseID)
	}

	inputItems, err := mapMessagesToInputItems(tail)
	if err != nil {
		return responses.ResponseNewParams{}, err
	}
	if len(inputItems) > 0 {
		params.Input = responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam(inputItems),
		}
	}

	tools, err := mapTools(req.Tools)
	if err != nil {
		return responses.ResponseNewParams{}, err
	}
	if len(tools) > 0 {
		params.Tools = tools
	}

	return params, nil
}

func splitMessagesForContinuation(messages []domain.Message) (string, []domain.Message) {
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message.Role != domain.RoleAssistant {
			continue
		}

		responseID := strings.TrimSpace(message.ResponseID)
		if responseID == "" {
			continue
		}
		return responseID, messages[i+1:]
	}
	return "", messages
}

func mapMessagesToInputItems(messages []domain.Message) ([]responses.ResponseInputItemUnionParam, error) {
	items := make([]responses.ResponseInputItemUnionParam, 0, len(messages))
	for i, message := range messages {
		mapped, err := mapMessageToInputItems(message)
		if err != nil {
			return nil, fmt.Errorf("openai provider: map message[%d]: %w", i, err)
		}
		items = append(items, mapped...)
	}
	return items, nil
}

func mapMessageToInputItems(message domain.Message) ([]responses.ResponseInputItemUnionParam, error) {
	role := strings.TrimSpace(message.Role)
	switch role {
	case domain.RoleUser:
		if strings.TrimSpace(message.Content) == "" {
			return nil, nil
		}
		return []responses.ResponseInputItemUnionParam{
			responses.ResponseInputItemParamOfMessage(
				message.Content,
				responses.EasyInputMessageRoleUser,
			),
		}, nil

	case domain.RoleSystem:
		if strings.TrimSpace(message.Content) == "" {
			return nil, nil
		}
		return []responses.ResponseInputItemUnionParam{
			responses.ResponseInputItemParamOfMessage(
				message.Content,
				responses.EasyInputMessageRoleSystem,
			),
		}, nil

	case domain.RoleAssistant:
		items := make([]responses.ResponseInputItemUnionParam, 0, len(message.ToolCalls)+1)
		if strings.TrimSpace(message.Content) != "" {
			items = append(items, responses.ResponseInputItemParamOfMessage(
				message.Content,
				responses.EasyInputMessageRoleAssistant,
			))
		}
		for i, call := range message.ToolCalls {
			callID := strings.TrimSpace(call.ID)
			name := strings.TrimSpace(call.Name)
			if callID == "" {
				return nil, fmt.Errorf("assistant tool_call[%d]: id is empty", i)
			}
			if name == "" {
				return nil, fmt.Errorf("assistant tool_call[%d]: name is empty", i)
			}
			items = append(items, responses.ResponseInputItemParamOfFunctionCall(
				call.Arguments,
				callID,
				name,
			))
		}
		return items, nil

	case domain.RoleTool:
		callID := strings.TrimSpace(message.ToolCallID)
		if callID == "" {
			return nil, errors.New("tool message: tool_call_id is empty")
		}
		output, err := encodeToolOutput(message)
		if err != nil {
			return nil, err
		}
		return []responses.ResponseInputItemUnionParam{
			responses.ResponseInputItemParamOfFunctionCallOutput(callID, output),
		}, nil
	}

	return nil, fmt.Errorf("unsupported message role %q", role)
}

func mapTools(specs []domain.ToolSpec) ([]responses.ToolUnionParam, error) {
	if len(specs) == 0 {
		return nil, nil
	}

	tools := make([]responses.ToolUnionParam, 0, len(specs))
	for i, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			return nil, fmt.Errorf("tool[%d]: name is empty", i)
		}

		tool := responses.FunctionToolParam{
			Name:       name,
			Parameters: cloneMap(spec.Schema),
		}
		if description := strings.TrimSpace(spec.Description); description != "" {
			tool.Description = param.NewOpt(description)
		}
		tools = append(tools, responses.ToolUnionParam{OfFunction: &tool})
	}

	return tools, nil
}

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

func cloneMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}

	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
