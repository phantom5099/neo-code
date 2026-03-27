package bootstrap

import (
	"path/filepath"
	"testing"

	"neo-code/internal/config"
)

func TestNewProgramReturnsErrorWhenGlobalConfigMissing(t *testing.T) {
	origGlobalConfig := config.GlobalAppConfig
	t.Cleanup(func() { config.GlobalAppConfig = origGlobalConfig })

	config.GlobalAppConfig = nil

	p, err := NewProgram(4, "config.yaml", "D:/neo-code")
	if err == nil {
		t.Fatalf("expected error, got program %+v", p)
	}
}

func TestNewProgramBuildsBubbleTeaProgram(t *testing.T) {
	origGlobalConfig := config.GlobalAppConfig
	t.Cleanup(func() { config.GlobalAppConfig = origGlobalConfig })

	cfg := config.DefaultAppConfig()
	cfg.Memory.StoragePath = filepath.Join(t.TempDir(), "memory.json")
	config.GlobalAppConfig = cfg
	t.Setenv(cfg.APIKeyEnvVarName(), "secret")

	p, err := NewProgram(4, "config.yaml", "D:/neo-code")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil program")
	}
}
