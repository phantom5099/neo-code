package context

import (
	"testing"
)

func TestDefaultPinCheckerMatchesKeyArtifacts(t *testing.T) {
	t.Parallel()

	checker := NewDefaultPinChecker()

	tests := []struct {
		path     string
		expected bool
	}{
		{"README.md", true},
		{"README.txt", true},
		{"readme.md", false}, // glob 区分大小写
		{"api.spec.yaml", true},
		{"design.spec.md", true},
		{"db.schema.json", true},
		{"schema.sql", false}, // *schema.* 需要两端有内容
		{"db.schema.sql", true},
		{"docker-compose.yml", true},
		{"docker-compose.yaml", true},
		{".env", true},
		{".env.local", true},
		{".env.example", true},
		{"01_migration.sql", true},
		{"migration.rb", true},
		{"create_users_migration.sql", true},
		{"Makefile", true},
		{"go.mod", true},
		{"package.json", true},
		{"main.go", false},
		{"app.tsx", false},
		{"index.js", false},
		{"utils.py", false},
		{"style.css", false},
	}

	for _, tt := range tests {
		got := checker.ShouldPin("filesystem_write_file", map[string]string{"path": "/project/" + tt.path})
		if got != tt.expected {
			t.Errorf("ShouldPin(path=%q) = %v, want %v", tt.path, got, tt.expected)
		}
	}
}

func TestDefaultPinCheckerUsesRelativePath(t *testing.T) {
	t.Parallel()

	checker := NewDefaultPinChecker()

	// relative_path 优先于 path
	got := checker.ShouldPin("filesystem_write_file", map[string]string{
		"relative_path": "api.spec.yaml",
	})
	if !got {
		t.Error("expected relative_path match for api.spec.yaml")
	}
}

func TestDefaultPinCheckerFallsBackToPath(t *testing.T) {
	t.Parallel()

	checker := NewDefaultPinChecker()

	got := checker.ShouldPin("filesystem_write_file", map[string]string{
		"path": "/project/README.md",
	})
	if !got {
		t.Error("expected path fallback match for README.md")
	}
}

func TestDefaultPinCheckerNoPathReturnsFalse(t *testing.T) {
	t.Parallel()

	checker := NewDefaultPinChecker()

	got := checker.ShouldPin("filesystem_write_file", map[string]string{"workdir": "/tmp"})
	if got {
		t.Error("expected false when no path in metadata")
	}
}

func TestDefaultPinCheckerEmptyMetadataReturnsFalse(t *testing.T) {
	t.Parallel()

	checker := NewDefaultPinChecker()

	got := checker.ShouldPin("filesystem_write_file", nil)
	if got {
		t.Error("expected false for nil metadata")
	}
}

func TestDefaultPinCheckerBashToolNotPinned(t *testing.T) {
	t.Parallel()

	checker := NewDefaultPinChecker()

	// bash 工具元信息只有 workdir，不应被钉住
	got := checker.ShouldPin("bash", map[string]string{"workdir": "/project"})
	if got {
		t.Error("expected bash tool with workdir only to not be pinned")
	}
}
