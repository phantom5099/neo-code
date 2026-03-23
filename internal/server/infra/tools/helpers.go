package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const WorkspaceEnvVar = "NEOCODE_WORKSPACE"

var (
	workspaceRootMu    sync.RWMutex
	configuredRootPath string
)

func requiredString(params map[string]interface{}, key string) (string, *ToolResult) {
	value, ok := params[key]
	if !ok {
		return "", &ToolResult{Success: false, Error: fmt.Sprintf("缺少必需参数: %s", key)}
	}
	str, ok := value.(string)
	if !ok || strings.TrimSpace(str) == "" {
		return "", &ToolResult{Success: false, Error: fmt.Sprintf("%s 必须是非空字符串", key)}
	}
	return str, nil
}

func optionalString(params map[string]interface{}, key, fallback string) (string, *ToolResult) {
	value, ok := params[key]
	if !ok {
		return fallback, nil
	}
	str, ok := value.(string)
	if !ok {
		return "", &ToolResult{Success: false, Error: fmt.Sprintf("%s 必须是字符串", key)}
	}
	if strings.TrimSpace(str) == "" {
		return fallback, nil
	}
	return str, nil
}

func optionalInt(params map[string]interface{}, key string, fallback int) (int, *ToolResult) {
	value, ok := params[key]
	if !ok {
		return fallback, nil
	}
	switch v := value.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case string:
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return 0, &ToolResult{Success: false, Error: fmt.Sprintf("%s 必须是数字", key)}
		}
		return parsed, nil
	default:
		return 0, &ToolResult{Success: false, Error: fmt.Sprintf("%s 必须是数字", key)}
	}
}

func optionalBool(params map[string]interface{}, key string, fallback bool) (bool, *ToolResult) {
	value, ok := params[key]
	if !ok {
		return fallback, nil
	}
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes":
			return true, nil
		case "false", "0", "no":
			return false, nil
		default:
			return false, &ToolResult{Success: false, Error: fmt.Sprintf("%s 必须是布尔值", key)}
		}
	default:
		return false, &ToolResult{Success: false, Error: fmt.Sprintf("%s 必须是布尔值", key)}
	}
}

// ResolveWorkspaceRoot 解析工作区根目录。
// 优先级：cliOverride > NEOCODE_WORKSPACE > 当前进程工作目录。
func ResolveWorkspaceRoot(cliOverride string) (string, error) {
	candidate := strings.TrimSpace(cliOverride)
	if candidate == "" {
		candidate = strings.TrimSpace(os.Getenv(WorkspaceEnvVar))
	}
	if candidate == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("获取当前工作目录失败: %w", err)
		}
		candidate = wd
	}

	absPath, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("解析工作区绝对路径失败: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("读取工作区路径失败: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("工作区路径不是目录: %s", absPath)
	}
	return absPath, nil
}

// SetWorkspaceRoot 固定工具层使用的工作区根目录。
func SetWorkspaceRoot(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return fmt.Errorf("工作区路径不能为空")
	}
	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return fmt.Errorf("解析工作区绝对路径失败: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("读取工作区路径失败: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("工作区路径不是目录: %s", absPath)
	}

	workspaceRootMu.Lock()
	configuredRootPath = absPath
	workspaceRootMu.Unlock()
	return nil
}

// GetWorkspaceRoot 返回当前固定的工作区根目录。
// 若尚未设置，则回退到当前进程工作目录。
func GetWorkspaceRoot() string {
	workspaceRootMu.RLock()
	root := configuredRootPath
	workspaceRootMu.RUnlock()
	if root != "" {
		return root
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	absPath, err := filepath.Abs(wd)
	if err != nil {
		return wd
	}
	return absPath
}

func workspaceRoot() string {
	return GetWorkspaceRoot()
}

func resolveWorkspacePath(path string) (string, error) {
	root := workspaceRoot()
	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	candidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", fmt.Errorf("路径超出工作区: %s", path)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("路径超出工作区: %s", path)
	}
	return candidate, nil
}

func ensureWorkspacePath(path string) (string, *ToolResult) {
	resolved, err := resolveWorkspacePath(path)
	if err != nil {
		return "", &ToolResult{Success: false, Error: err.Error()}
	}
	return resolved, nil
}

// AtomicWrite 以原子方式将内容写入文件，确保文件完整性。
func AtomicWrite(filePath string, content []byte) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 1. 在同目录下创建临时文件
	tmpFile, err := os.CreateTemp(dir, "neocode-tmp-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath) // 正常 Rename 后 Remove 会失败，属正常行为
	}()

	// 2. 写入内容
	if _, err := tmpFile.Write(content); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}

	// 3. 强制刷盘，确保操作系统已将数据写入物理存储
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("刷盘失败: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}

	// 4. 原子重命名，替换目标文件
	// 在 Unix-like 和 Windows (同一卷内) 这通常是原子的
	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("原子替换失败: %w", err)
	}

	return nil
}

func NormalizeParams(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return map[string]interface{}{}
	}
	result := make(map[string]interface{}, len(params))
	for key, value := range params {
		camelKey := snakeToCamel(strings.TrimSpace(key))
		switch typed := value.(type) {
		case map[string]interface{}:
			result[camelKey] = NormalizeParams(typed)
		default:
			result[camelKey] = value
		}
	}
	return result
}

func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	if len(parts) <= 1 {
		return s
	}
	out := parts[0]
	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		out += strings.ToUpper(p[:1]) + p[1:]
	}
	return out
}
