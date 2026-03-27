package web

import (
	"context"
	"fmt"
	"strings"

	"neo-code/internal/tool"
)

type FetchTool struct{}

func NewFetchTool() *FetchTool {
	return &FetchTool{}
}

func (w *FetchTool) Definition() tool.ToolDefinition {
	return tool.ToolDefinition{
		Category:    "web",
		Name:        "webfetch",
		Description: "Fetch an HTTP or HTTPS resource and return a truncated response body with status metadata.",
		Parameters: []tool.ToolParamSpec{
			{Name: "url", Type: "string", Required: true, Description: "Target HTTP or HTTPS URL."},
			{Name: "maxBytes", Type: "integer", Description: "Maximum response bytes to read, default 20000."},
		},
	}
}

func (w *FetchTool) Run(params map[string]interface{}) *tool.ToolResult {
	rawURL, errRes := tool.RequiredString(params, "url")
	if errRes != nil {
		errRes.ToolName = w.Definition().Name
		return errRes
	}

	parsed, err := NormalizeHTTPURL(rawURL)
	if err != nil {
		return &tool.ToolResult{ToolName: w.Definition().Name, Success: false, Error: err.Error()}
	}
	target := strings.TrimSpace(parsed.Hostname())
	if denied := tool.GuardToolExecution(string(tool.ToolWebFetch), target, w.Definition().Name); denied != nil {
		return denied
	}

	maxBytes, errRes := tool.OptionalInt(params, "maxBytes", 20_000)
	if errRes != nil {
		errRes.ToolName = w.Definition().Name
		return errRes
	}

	result, err := Fetch(context.Background(), nil, parsed.String(), maxBytes)
	if err != nil {
		return &tool.ToolResult{ToolName: w.Definition().Name, Success: false, Error: err.Error()}
	}

	return &tool.ToolResult{
		ToolName: w.Definition().Name,
		Success:  true,
		Output:   result.Body,
		Metadata: map[string]interface{}{
			"url":         result.URL,
			"statusCode":  result.StatusCode,
			"contentType": result.ContentType,
			"maxBytes":    maxBytes,
			"summary":     fmt.Sprintf("%d %s", result.StatusCode, result.ContentType),
		},
	}
}
