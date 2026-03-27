package tool

import (
	"context"
	"fmt"
	"strings"

	webclient "neo-code/internal/tool/web"
)

type WebFetchTool struct{}

func (w *WebFetchTool) Definition() ToolDefinition {
	return ToolDefinition{
		Category:    "web",
		Name:        "webfetch",
		Description: "Fetch an HTTP or HTTPS resource and return a truncated response body with status metadata.",
		Parameters: []ToolParamSpec{
			{Name: "url", Type: "string", Required: true, Description: "Target HTTP or HTTPS URL."},
			{Name: "maxBytes", Type: "integer", Description: "Maximum response bytes to read, default 20000."},
		},
	}
}

func (w *WebFetchTool) Run(params map[string]interface{}) *ToolResult {
	rawURL, errRes := requiredString(params, "url")
	if errRes != nil {
		errRes.ToolName = w.Definition().Name
		return errRes
	}

	parsed, err := webclient.NormalizeHTTPURL(rawURL)
	if err != nil {
		return &ToolResult{ToolName: w.Definition().Name, Success: false, Error: err.Error()}
	}
	target := strings.TrimSpace(parsed.Hostname())
	if denied := guardToolExecution(string(ToolWebFetch), target, w.Definition().Name); denied != nil {
		return denied
	}

	maxBytes, errRes := optionalInt(params, "maxBytes", 20_000)
	if errRes != nil {
		errRes.ToolName = w.Definition().Name
		return errRes
	}

	result, err := webclient.Fetch(context.Background(), nil, parsed.String(), maxBytes)
	if err != nil {
		return &ToolResult{ToolName: w.Definition().Name, Success: false, Error: err.Error()}
	}

	return &ToolResult{
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
