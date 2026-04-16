package security

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCapabilitySignerRoundTripAndTamper(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	token := CapabilityToken{
		ID:              "token-1",
		TaskID:          "task-1",
		AgentID:         "agent-1",
		IssuedAt:        now.Add(-time.Minute),
		ExpiresAt:       now.Add(time.Hour),
		AllowedTools:    []string{"filesystem_read_file"},
		AllowedPaths:    []string{"/workspace"},
		NetworkPolicy:   NetworkPolicy{Mode: NetworkPermissionDenyAll},
		WritePermission: WritePermissionWorkspace,
	}

	signer, err := NewCapabilitySigner([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	signed, err := signer.Sign(token)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if signed.Signature == "" {
		t.Fatalf("expected non-empty signature")
	}
	if err := signer.Verify(signed); err != nil {
		t.Fatalf("verify signed token: %v", err)
	}

	tampered := signed
	tampered.AllowedTools = []string{"filesystem_write_file"}
	if err := signer.Verify(tampered); err == nil || !strings.Contains(err.Error(), "signature mismatch") {
		t.Fatalf("expected signature mismatch for tampered token, got %v", err)
	}
}

func TestEvaluateCapabilityAction(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	workdir := t.TempDir()
	allowedRoot := filepath.Join(workdir, "allowed")
	token := CapabilityToken{
		ID:              "token-2",
		TaskID:          "task-2",
		AgentID:         "agent-2",
		IssuedAt:        now.Add(-time.Minute),
		ExpiresAt:       now.Add(time.Hour),
		AllowedTools:    []string{"filesystem_read_file", "webfetch"},
		AllowedPaths:    []string{allowedRoot},
		NetworkPolicy:   NetworkPolicy{Mode: NetworkPermissionAllowHosts, AllowedHosts: []string{"example.com"}},
		WritePermission: WritePermissionNone,
	}

	tests := []struct {
		name      string
		action    Action
		wantAllow bool
		wantInErr string
	}{
		{
			name: "allow read in allowed path",
			action: Action{
				Type: ActionTypeRead,
				Payload: ActionPayload{
					ToolName:          "filesystem_read_file",
					Resource:          "filesystem_read_file",
					Workdir:           workdir,
					TargetType:        TargetTypePath,
					Target:            "allowed/readme.md",
					SandboxTargetType: TargetTypePath,
					SandboxTarget:     "allowed/readme.md",
				},
			},
			wantAllow: true,
		},
		{
			name: "deny traversal path",
			action: Action{
				Type: ActionTypeRead,
				Payload: ActionPayload{
					ToolName:          "filesystem_read_file",
					Resource:          "filesystem_read_file",
					Workdir:           workdir,
					TargetType:        TargetTypePath,
					Target:            "../outside.txt",
					SandboxTargetType: TargetTypePath,
					SandboxTarget:     "../outside.txt",
				},
			},
			wantAllow: false,
			wantInErr: "traversal",
		},
		{
			name: "deny tool miss",
			action: Action{
				Type: ActionTypeRead,
				Payload: ActionPayload{
					ToolName: "filesystem_glob",
					Resource: "filesystem_glob",
				},
			},
			wantAllow: false,
			wantInErr: "tool not allowed",
		},
		{
			name: "deny network host miss",
			action: Action{
				Type: ActionTypeRead,
				Payload: ActionPayload{
					ToolName:   "webfetch",
					Resource:   "webfetch",
					TargetType: TargetTypeURL,
					Target:     "https://not-example.com/path",
				},
			},
			wantAllow: false,
			wantInErr: "host not allowed",
		},
		{
			name: "deny write by write permission",
			action: Action{
				Type: ActionTypeWrite,
				Payload: ActionPayload{
					ToolName:          "filesystem_read_file",
					Resource:          "filesystem_read_file",
					Workdir:           workdir,
					TargetType:        TargetTypePath,
					Target:            "allowed/readme.md",
					SandboxTargetType: TargetTypePath,
					SandboxTarget:     "allowed/readme.md",
				},
			},
			wantAllow: false,
			wantInErr: "write permission denied",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			allowed, reason := EvaluateCapabilityAction(token, tt.action, now)
			if allowed != tt.wantAllow {
				t.Fatalf("allow=%v, want %v, reason=%q", allowed, tt.wantAllow, reason)
			}
			if tt.wantInErr != "" && !strings.Contains(strings.ToLower(reason), strings.ToLower(tt.wantInErr)) {
				t.Fatalf("expected reason to contain %q, got %q", tt.wantInErr, reason)
			}
		})
	}
}

func TestEnsureCapabilitySubset(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	parent := CapabilityToken{
		ID:              "parent",
		TaskID:          "task",
		AgentID:         "agent-parent",
		IssuedAt:        now.Add(-time.Minute),
		ExpiresAt:       now.Add(2 * time.Hour),
		AllowedTools:    []string{"filesystem_read_file", "webfetch"},
		AllowedPaths:    []string{"/workspace", "/workspace/sub"},
		NetworkPolicy:   NetworkPolicy{Mode: NetworkPermissionAllowHosts, AllowedHosts: []string{"example.com", "*.github.com"}},
		WritePermission: WritePermissionWorkspace,
	}

	tests := []struct {
		name      string
		child     CapabilityToken
		wantError string
	}{
		{
			name: "subset allowed",
			child: CapabilityToken{
				ID:              "child-ok",
				TaskID:          "task",
				AgentID:         "agent-child",
				IssuedAt:        now.Add(-time.Minute),
				ExpiresAt:       now.Add(time.Hour),
				AllowedTools:    []string{"filesystem_read_file"},
				AllowedPaths:    []string{"/workspace/sub"},
				NetworkPolicy:   NetworkPolicy{Mode: NetworkPermissionAllowHosts, AllowedHosts: []string{"example.com"}},
				WritePermission: WritePermissionNone,
			},
		},
		{
			name: "deny broader tool",
			child: CapabilityToken{
				ID:              "child-tool",
				TaskID:          "task",
				AgentID:         "agent-child",
				IssuedAt:        now.Add(-time.Minute),
				ExpiresAt:       now.Add(time.Hour),
				AllowedTools:    []string{"filesystem_read_file", "filesystem_write_file"},
				AllowedPaths:    []string{"/workspace/sub"},
				NetworkPolicy:   NetworkPolicy{Mode: NetworkPermissionAllowHosts, AllowedHosts: []string{"example.com"}},
				WritePermission: WritePermissionNone,
			},
			wantError: "allowed_tools exceeds parent",
		},
		{
			name: "deny longer ttl",
			child: CapabilityToken{
				ID:              "child-ttl",
				TaskID:          "task",
				AgentID:         "agent-child",
				IssuedAt:        now.Add(-time.Minute),
				ExpiresAt:       now.Add(3 * time.Hour),
				AllowedTools:    []string{"filesystem_read_file"},
				AllowedPaths:    []string{"/workspace/sub"},
				NetworkPolicy:   NetworkPolicy{Mode: NetworkPermissionAllowHosts, AllowedHosts: []string{"example.com"}},
				WritePermission: WritePermissionNone,
			},
			wantError: "expires_at exceeds parent",
		},
		{
			name: "deny broader network",
			child: CapabilityToken{
				ID:              "child-net",
				TaskID:          "task",
				AgentID:         "agent-child",
				IssuedAt:        now.Add(-time.Minute),
				ExpiresAt:       now.Add(time.Hour),
				AllowedTools:    []string{"filesystem_read_file"},
				AllowedPaths:    []string{"/workspace/sub"},
				NetworkPolicy:   NetworkPolicy{Mode: NetworkPermissionAllowAll},
				WritePermission: WritePermissionNone,
			},
			wantError: "network policy exceeds parent",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := EnsureCapabilitySubset(parent, tt.child)
			if tt.wantError == "" && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
				}
			}
		})
	}
}

func TestEvaluateCapabilityForEngine(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	token := CapabilityToken{
		ID:              "token-3",
		TaskID:          "task-3",
		AgentID:         "agent-3",
		IssuedAt:        now.Add(-time.Minute),
		ExpiresAt:       now.Add(time.Hour),
		AllowedTools:    []string{"filesystem_read_file"},
		AllowedPaths:    []string{"/workspace"},
		NetworkPolicy:   NetworkPolicy{Mode: NetworkPermissionDenyAll},
		WritePermission: WritePermissionWorkspace,
	}
	action := Action{
		Type: ActionTypeRead,
		Payload: ActionPayload{
			ToolName:        "filesystem_glob",
			Resource:        "filesystem_glob",
			CapabilityToken: &token,
		},
	}

	result, denied := EvaluateCapabilityForEngine(action, now)
	if !denied {
		t.Fatalf("expected denied result")
	}
	if result.Decision != DecisionDeny {
		t.Fatalf("expected deny decision, got %q", result.Decision)
	}
	if result.Rule == nil || result.Rule.ID != CapabilityRuleID {
		t.Fatalf("expected capability rule id, got %+v", result.Rule)
	}
	if !IsCapabilityDeniedResult(result) {
		t.Fatalf("expected IsCapabilityDeniedResult to be true")
	}
}

func TestNormalizePathKeyPlatformSemantics(t *testing.T) {
	t.Parallel()

	raw := filepath.Join("Workspace", "Sub", "..", "File.txt")
	got := normalizePathKey(raw)
	if got == "" {
		t.Fatalf("expected normalized path key, got empty")
	}

	upper := normalizePathKey(filepath.Join("Workspace", "File.txt"))
	lower := normalizePathKey(filepath.Join("workspace", "file.txt"))

	if runtime.GOOS == "windows" {
		if upper != lower {
			t.Fatalf("windows path key should ignore case: %q vs %q", upper, lower)
		}
		return
	}
	if upper == lower {
		t.Fatalf("non-windows path key should keep case sensitivity: %q vs %q", upper, lower)
	}
}
