package protocol

import (
	"strings"
	"testing"

	"neo-code/internal/tool"
)

func TestParseAssistantToolCallsSupportsStandardEnvelope(t *testing.T) {
	calls := ParseAssistantToolCalls(`{"type":"tool_call","name":"read","arguments":{"file_path":"README.md"}}`)
	if len(calls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(calls))
	}
	if calls[0].Tool != "read" {
		t.Fatalf("expected read tool, got %q", calls[0].Tool)
	}
	if calls[0].Params["filePath"] != "README.md" {
		t.Fatalf("expected normalized filePath argument, got %+v", calls[0].Params)
	}
}

func TestParseAssistantToolCallsSupportsLegacyEnvelope(t *testing.T) {
	calls := ParseAssistantToolCalls("```json\n{\"tool\":\"bash\",\"params\":{\"command\":\"go test ./...\"}}\n```")
	if len(calls) != 1 {
		t.Fatalf("expected one legacy tool call, got %d", len(calls))
	}
	if calls[0].Tool != "bash" {
		t.Fatalf("expected bash tool, got %q", calls[0].Tool)
	}
	if calls[0].Params["command"] != "go test ./..." {
		t.Fatalf("expected command argument, got %+v", calls[0].Params)
	}
}

func TestRenderInstructionBlockIncludesSchema(t *testing.T) {
	text := RenderInstructionBlock([]tool.ToolDefinition{
		{
			Name:        "read",
			Description: "Read a file.",
			Parameters: []tool.ToolParamSpec{
				{Name: "filePath", Type: "string", Required: true, Description: "Target file path."},
			},
		},
	})

	for _, want := range []string{"[TOOLS]", "\"type\":\"tool_call\"", "read", "\"filePath\"", "\"required\":[\"filePath\"]"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected instruction block to contain %q, got %q", want, text)
		}
	}
}
