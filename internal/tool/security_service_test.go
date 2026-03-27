package tool

import (
	"os"
	"path/filepath"
	"testing"

	toolsecurity "neo-code/internal/tool/security"
)

func TestSecurityServiceCheck(t *testing.T) {
	configDir := t.TempDir()
	mustWrite := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(configDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write config %s: %v", name, err)
		}
	}

	mustWrite("blacklist.yaml", "rules:\n  - target: .git/**\n    read: deny\n    write: deny\n  - target: '**/.env'\n    read: deny\n  - command: rm -rf **\n    exec: deny\n")
	mustWrite("whitelist.yaml", "rules:\n  - target: src/**/*.go\n    read: allow\n  - command: go version\n    exec: allow\n  - domain: '*.google.com'\n    network: allow\n")
	mustWrite("yellowlist.yaml", "rules:\n  - target: src/**/*.go\n    write: ask\n  - command: go build **\n    exec: ask\n")

	svc, err := toolsecurity.LoadPolicy(configDir)
	if err != nil {
		t.Fatalf("LoadPolicy() error = %v", err)
	}

	tests := []struct {
		name     string
		toolType string
		target   string
		want     Action
	}{
		{name: "blacklist read", toolType: string(ToolRead), target: ".git/config", want: ActionDeny},
		{name: "blacklist write", toolType: string(ToolWrite), target: ".git/HEAD", want: ActionDeny},
		{name: "bash deny", toolType: string(ToolBash), target: "rm -rf /tmp/build", want: ActionDeny},
		{name: "relative traversal deny", toolType: string(ToolRead), target: "../secrets.txt", want: ActionDeny},
		{name: "normalized traversal deny", toolType: string(ToolRead), target: "src/../.git/config", want: ActionDeny},
		{name: "allow source read", toolType: string(ToolRead), target: "src/main.go", want: ActionAllow},
		{name: "allow nested source read", toolType: string(ToolRead), target: "src/internal/pkg/util.go", want: ActionAllow},
		{name: "ask source write", toolType: string(ToolWrite), target: "src/main.go", want: ActionAsk},
		{name: "allow bash", toolType: string(ToolBash), target: "go version", want: ActionAllow},
		{name: "ask bash", toolType: string(ToolBash), target: "go build ./...", want: ActionAsk},
		{name: "allow webfetch", toolType: string(ToolWebFetch), target: "api.google.com", want: ActionAllow},
		{name: "default ask", toolType: "SelfDestruct", target: "now", want: ActionAsk},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.Check(tt.toolType, tt.target); got != tt.want {
				t.Fatalf("Check(%q, %q) = %q, want %q", tt.toolType, tt.target, got, tt.want)
			}
		})
	}
}
