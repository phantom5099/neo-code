package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const WorkspaceEnvVar = "NEOCODE_WORKSPACE"

var (
	workspaceRootMu    sync.RWMutex
	configuredRootPath string
)

func ResolveWorkspaceRoot(cliOverride string) (string, error) {
	candidate := strings.TrimSpace(cliOverride)
	if candidate == "" {
		candidate = strings.TrimSpace(os.Getenv(WorkspaceEnvVar))
	}
	if candidate == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve working directory: %w", err)
		}
		candidate = wd
	}

	absPath, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("stat workspace root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace root must be a directory: %s", absPath)
	}

	return absPath, nil
}

func SetWorkspaceRoot(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return fmt.Errorf("workspace root cannot be empty")
	}

	root, err := ResolveWorkspaceRoot(trimmed)
	if err != nil {
		return err
	}

	workspaceRootMu.Lock()
	configuredRootPath = root
	workspaceRootMu.Unlock()
	return nil
}

func GetWorkspaceRoot() string {
	workspaceRootMu.RLock()
	root := configuredRootPath
	workspaceRootMu.RUnlock()
	if root != "" {
		return root
	}

	resolved, err := ResolveWorkspaceRoot("")
	if err != nil {
		return "."
	}
	return resolved
}

func workspaceRoot() string {
	return GetWorkspaceRoot()
}

func resolveWorkspacePath(path string) (string, error) {
	rootAbs, err := filepath.Abs(workspaceRoot())
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}

	candidate := strings.TrimSpace(path)
	if candidate == "" {
		candidate = "."
	}
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(rootAbs, candidate)
	}

	candidateAbs, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", path, err)
	}

	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return "", fmt.Errorf("path %q is outside the workspace", path)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q is outside the workspace", path)
	}

	return candidateAbs, nil
}

func ensureWorkspacePath(path string) (string, *ToolResult) {
	resolved, err := resolveWorkspacePath(path)
	if err != nil {
		return "", &ToolResult{Success: false, Error: err.Error()}
	}
	return resolved, nil
}

func EnsureWorkspacePath(path string) (string, *ToolResult) {
	return ensureWorkspacePath(path)
}

func AtomicWrite(filePath string, content []byte) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create parent directories: %w", err)
	}

	perm := os.FileMode(0o644)
	if info, err := os.Stat(filePath); err == nil {
		perm = info.Mode().Perm()
	}

	tmpFile, err := os.CreateTemp(dir, "neocode-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(content); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Chmod(perm); err != nil {
		return fmt.Errorf("set temp file permissions: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, filePath); err == nil {
		return nil
	}

	if removeErr := os.Remove(filePath); removeErr != nil && !os.IsNotExist(removeErr) {
		return fmt.Errorf("replace destination file: %w", removeErr)
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("replace destination file: %w", err)
	}

	return nil
}
