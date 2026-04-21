package tools

import (
	"testing"

	"neo-code/internal/security"
)

func TestEnrichToolResultFactsDefaultsFromAction(t *testing.T) {
	t.Parallel()

	read := EnrichToolResultFacts(security.Action{Type: security.ActionTypeRead}, ToolResult{})
	if read.Facts.WorkspaceWrite {
		t.Fatalf("expected read action to default workspace_write=false")
	}

	bash := EnrichToolResultFacts(security.Action{Type: security.ActionTypeBash}, ToolResult{})
	if !bash.Facts.WorkspaceWrite {
		t.Fatalf("expected bash action to default workspace_write=true")
	}

	mcp := EnrichToolResultFacts(security.Action{Type: security.ActionTypeMCP}, ToolResult{})
	if !mcp.Facts.WorkspaceWrite {
		t.Fatalf("expected mcp action to default workspace_write=true")
	}
}

func TestEnrichToolResultFactsRespectsExplicitMetadata(t *testing.T) {
	t.Parallel()

	result := EnrichToolResultFacts(
		security.Action{Type: security.ActionTypeMCP},
		ToolResult{
			Metadata: map[string]any{
				"workspace_write":        false,
				"verification_performed": true,
				"verification_passed":    true,
				"verification_scope":     "workspace",
			},
		},
	)
	if result.Facts.WorkspaceWrite {
		t.Fatalf("expected explicit workspace_write=false to override default")
	}
	if !result.Facts.VerificationPerformed || !result.Facts.VerificationPassed {
		t.Fatalf("expected verification facts to be populated, got %+v", result.Facts)
	}
	if result.Facts.VerificationScope != "workspace" {
		t.Fatalf("verification scope = %q, want workspace", result.Facts.VerificationScope)
	}
}
