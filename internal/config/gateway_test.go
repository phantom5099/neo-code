package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGatewayConfigDefaultsAndClone(t *testing.T) {
	t.Parallel()

	defaults := defaultGatewayConfig()
	if defaults.Security.ACLMode != DefaultGatewayACLMode {
		t.Fatalf("acl_mode = %q, want %q", defaults.Security.ACLMode, DefaultGatewayACLMode)
	}
	if defaults.Limits.MaxFrameBytes != DefaultGatewayMaxFrameBytes {
		t.Fatalf("max_frame_bytes = %d, want %d", defaults.Limits.MaxFrameBytes, DefaultGatewayMaxFrameBytes)
	}
	if !defaults.Observability.Enabled() {
		t.Fatal("metrics should be enabled by default")
	}

	cloned := defaults.Clone()
	cloned.Security.AllowOrigins[0] = "http://changed"
	if defaults.Security.AllowOrigins[0] == "http://changed" {
		t.Fatal("clone should not share allow_origins slice")
	}
}

func TestGatewayConfigApplyDefaultsAndValidate(t *testing.T) {
	t.Parallel()

	cfg := GatewayConfig{}
	cfg.ApplyDefaults(defaultGatewayConfig())
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate defaulted gateway config: %v", err)
	}

	cfg.Observability.MetricsEnabled = boolPtr(false)
	cfg.ApplyDefaults(defaultGatewayConfig())
	if cfg.Observability.Enabled() {
		t.Fatal("explicit metrics_enabled=false should be preserved")
	}

	invalid := cfg.Clone()
	invalid.Security.ACLMode = "allow-all"
	if err := invalid.Validate(); err == nil || !strings.Contains(err.Error(), "acl_mode") {
		t.Fatalf("expected acl_mode error, got %v", err)
	}
}

func TestLoadGatewayConfig(t *testing.T) {
	t.Parallel()

	t.Run("missing file uses defaults", func(t *testing.T) {
		t.Parallel()
		cfg, err := LoadGatewayConfig(context.Background(), t.TempDir())
		if err != nil {
			t.Fatalf("load gateway config: %v", err)
		}
		if !cfg.Observability.Enabled() {
			t.Fatal("metrics should default to enabled")
		}
	})

	t.Run("reads gateway section", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		configPath := filepath.Join(baseDir, configName)
		content := `
selected_provider: openai
current_model: gpt-5.4
shell: bash
gateway:
  security:
    acl_mode: strict
    token_file: /tmp/neocode-auth.json
    allow_origins:
      - http://localhost
      - app://
  limits:
    max_frame_bytes: 2048
    ipc_max_connections: 32
    http_max_request_bytes: 4096
    http_max_stream_connections: 16
  timeouts:
    ipc_read_sec: 20
    ipc_write_sec: 21
    http_read_sec: 9
    http_write_sec: 10
    http_shutdown_sec: 4
  observability:
    metrics_enabled: false
`
		if err := os.WriteFile(configPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		cfg, err := LoadGatewayConfig(context.Background(), baseDir)
		if err != nil {
			t.Fatalf("load gateway config: %v", err)
		}
		if cfg.Limits.MaxFrameBytes != 2048 {
			t.Fatalf("max_frame_bytes = %d, want %d", cfg.Limits.MaxFrameBytes, 2048)
		}
		if cfg.Observability.Enabled() {
			t.Fatal("metrics_enabled should be false")
		}
		if cfg.Security.TokenFile != "/tmp/neocode-auth.json" {
			t.Fatalf("token_file = %q, want %q", cfg.Security.TokenFile, "/tmp/neocode-auth.json")
		}
	})

	t.Run("invalid gateway section returns error", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		configPath := filepath.Join(baseDir, configName)
		content := `
selected_provider: openai
current_model: gpt-5.4
shell: bash
gateway:
  limits:
    max_frame_bytes: 0
`
		if err := os.WriteFile(configPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		cfg, err := LoadGatewayConfig(context.Background(), baseDir)
		if err != nil {
			t.Fatalf("load gateway config: %v", err)
		}
		if cfg.Limits.MaxFrameBytes != DefaultGatewayMaxFrameBytes {
			t.Fatalf("max_frame_bytes = %d, want fallback %d", cfg.Limits.MaxFrameBytes, DefaultGatewayMaxFrameBytes)
		}
	})
}
