package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var DefaultPersonaFilePath = DefaultPersonaPath()

const legacyPersonaFilePath = "./persona.txt"

const defaultPersonaPrompt = `你是 NeoCode，一个专业、可靠、简洁的 AI 编程助手。

要求：
- 优先给出可执行、可落地的工程方案。
- 回答简洁清晰，必要时给出分步建议。
- 对不确定的内容明确说明假设。
- 默认使用中文回答。

工具与安全要求：
- 工具列表、输入 schema 和调用协议会由系统上下文动态提供，必须严格遵守。
- 需要调用工具时，只输出结构化 tool call，不要混入额外解释文本。
- 对可能有副作用的操作先进行风险判断，再决定是否调用工具。
- 当安全策略为 deny 时，必须拒绝执行并说明原因。
- 当安全策略为 ask 时，必须等待用户确认后再继续。
- 路径操作默认限定在当前工作区，不访问工作区外路径。
- 不得绕过系统、安全策略或工作区边界。`

func ResolvePersonaFilePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}

	candidates := []string{trimmed}
	if trimmed == legacyPersonaFilePath || trimmed == "persona.txt" {
		candidates = append(
			candidates,
			"./internal/config/persona.txt",
			"internal/config/persona.txt",
			"./configs/persona.txt",
			"configs/persona.txt",
			DefaultPersonaFilePath,
		)
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return trimmed
}

func LoadPersonaPrompt(path string) (string, string, error) {
	resolvedPath := ResolvePersonaFilePath(path)
	if resolvedPath == "" {
		return "", "", nil
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", resolvedPath, fmt.Errorf("read persona file %q: %w", resolvedPath, err)
	}

	return strings.TrimSpace(string(data)), resolvedPath, nil
}

func ensureDefaultPersonaFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat persona file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create persona directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(defaultPersonaPrompt), 0o644); err != nil {
		return fmt.Errorf("write default persona file: %w", err)
	}
	return nil
}
