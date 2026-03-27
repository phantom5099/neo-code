package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"neo-code/internal/tool"
)

type GrepTool struct{}

func NewGrepTool() *GrepTool {
	return &GrepTool{}
}

func (g *GrepTool) Definition() tool.ToolDefinition {
	return tool.ToolDefinition{
		Category:    "filesystem",
		Name:        "grep",
		Description: "Recursively search for file content in the workspace using regular expressions and return all matches with file, line number, and text.",
		Parameters: []tool.ToolParamSpec{
			{Name: "pattern", Type: "string", Required: true, Description: "The regular expression to search for."},
			{Name: "path", Type: "string", Description: "Search root directory within the workspace, defaults to workspace root."},
			{Name: "include", Type: "string", Description: "Optional filename glob filter, e.g., '*.go'."},
		},
	}
}

func (g *GrepTool) Run(params map[string]interface{}) *tool.ToolResult {
	pattern, errRes := tool.RequiredString(params, "pattern")
	if errRes != nil {
		errRes.ToolName = g.Definition().Name
		return errRes
	}
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return &tool.ToolResult{ToolName: g.Definition().Name, Success: false, Error: fmt.Sprintf("invalid regex pattern: %v", err)}
	}
	searchPath, errRes := tool.OptionalString(params, "path", ".")
	if errRes != nil {
		errRes.ToolName = g.Definition().Name
		return errRes
	}
	searchPath, pathErr := tool.EnsureWorkspacePath(searchPath)
	if pathErr != nil {
		pathErr.ToolName = g.Definition().Name
		return pathErr
	}
	includePattern, errRes := tool.OptionalString(params, "include", "")
	if errRes != nil {
		errRes.ToolName = g.Definition().Name
		return errRes
	}

	var lines []string
	matchCount := 0
	walkErr := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if includePattern != "" {
			matched, matchErr := filepath.Match(includePattern, filepath.Base(path))
			if matchErr != nil {
				return matchErr
			}
			if !matched {
				return nil
			}
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		fileLines := regexp.MustCompile("\\r?\\n").Split(string(content), -1)
		for idx, line := range fileLines {
			if regex.MatchString(line) {
				matchCount++
				lines = append(lines, fmt.Sprintf("%s:%d:%s", path, idx+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	if walkErr != nil {
		return &tool.ToolResult{ToolName: g.Definition().Name, Success: false, Error: walkErr.Error()}
	}
	if matchCount == 0 {
		return &tool.ToolResult{ToolName: g.Definition().Name, Success: true, Output: "No matches found.", Metadata: map[string]interface{}{"pattern": pattern, "path": searchPath, "include": includePattern, "matches": 0}}
	}
	return &tool.ToolResult{ToolName: g.Definition().Name, Success: true, Output: strings.Join(lines, "\n") + "\n", Metadata: map[string]interface{}{"pattern": pattern, "path": searchPath, "include": includePattern, "matches": matchCount}}
}
