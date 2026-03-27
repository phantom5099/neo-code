package web

import (
	"context"
	"fmt"
	"strings"

	"neo-code/internal/tool"
)

type SearchTool struct{}

func NewSearchTool() *SearchTool {
	return &SearchTool{}
}

func (w *SearchTool) Definition() tool.ToolDefinition {
	return tool.ToolDefinition{
		Category:    "web",
		Name:        "websearch",
		Description: "Search the web and return a compact list of result titles and URLs. Supports an optional custom endpoint template.",
		Parameters: []tool.ToolParamSpec{
			{Name: "query", Type: "string", Required: true, Description: "Search query text."},
			{Name: "limit", Type: "integer", Description: "Maximum number of results to return, default 5."},
			{Name: "endpoint", Type: "string", Description: "Optional search endpoint template. Use {query} as the placeholder for the escaped query."},
		},
	}
}

func (w *SearchTool) Run(params map[string]interface{}) *tool.ToolResult {
	query, errRes := tool.RequiredString(params, "query")
	if errRes != nil {
		errRes.ToolName = w.Definition().Name
		return errRes
	}
	limit, errRes := tool.OptionalInt(params, "limit", 5)
	if errRes != nil {
		errRes.ToolName = w.Definition().Name
		return errRes
	}
	endpoint, errRes := tool.OptionalString(params, "endpoint", "")
	if errRes != nil {
		errRes.ToolName = w.Definition().Name
		return errRes
	}
	target, err := EndpointHost(strings.TrimSpace(endpoint))
	if strings.TrimSpace(endpoint) == "" {
		target = "duckduckgo.com"
		err = nil
	}
	if err != nil {
		return &tool.ToolResult{ToolName: w.Definition().Name, Success: false, Error: err.Error()}
	}
	if denied := tool.GuardToolExecution(string(tool.ToolWebFetch), target, w.Definition().Name); denied != nil {
		return denied
	}

	results, resolvedEndpoint, err := Search(context.Background(), nil, endpoint, query, limit)
	if err != nil {
		return &tool.ToolResult{ToolName: w.Definition().Name, Success: false, Error: err.Error()}
	}
	if len(results) == 0 {
		return &tool.ToolResult{
			ToolName: w.Definition().Name,
			Success:  true,
			Output:   "No search results found.",
			Metadata: map[string]interface{}{"query": query, "endpoint": resolvedEndpoint, "resultCount": 0},
		}
	}

	lines := make([]string, 0, len(results))
	for idx, item := range results {
		line := fmt.Sprintf("%d. %s\n%s", idx+1, item.Title, item.URL)
		if strings.TrimSpace(item.Snippet) != "" {
			line += "\n" + item.Snippet
		}
		lines = append(lines, line)
	}

	return &tool.ToolResult{
		ToolName: w.Definition().Name,
		Success:  true,
		Output:   strings.Join(lines, "\n\n"),
		Metadata: map[string]interface{}{"query": query, "endpoint": resolvedEndpoint, "resultCount": len(results)},
	}
}
