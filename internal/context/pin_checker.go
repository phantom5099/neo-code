package context

import (
	"path/filepath"
	"strings"
)

// defaultPinPatterns 列出关键产物文件的 basename glob 模式，匹配的工具结果不参与微压缩。
var defaultPinPatterns = []string{
	"README*",
	"*.spec.*",
	"*.schema.*",
	"docker-compose*",
	".env*",
	"*migration*",
	"Makefile",
	"go.mod",
	"package.json",
}

// pinChecker 基于文件路径 glob 模式判断工具结果是否应钉住。
type pinChecker struct {
	patterns []string
}

// NewDefaultPinChecker 返回使用默认钉住模式的 PinChecker。
func NewDefaultPinChecker() MicroCompactPinChecker {
	return &pinChecker{patterns: defaultPinPatterns}
}

// ShouldPin 判断工具结果是否应钉住：从 metadata 中提取文件路径，对 basename 匹配 glob 模式。
func (p *pinChecker) ShouldPin(toolName string, metadata map[string]string) bool {
	if len(metadata) == 0 {
		return false
	}

	path := metadata["relative_path"]
	if path == "" {
		path = metadata["path"]
	}
	if path == "" {
		return false
	}

	base := filepath.Base(path)
	for _, pattern := range p.patterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

// noopPinChecker 不钉住任何结果，用于测试和禁用场景。
type noopPinChecker struct{}

func (noopPinChecker) ShouldPin(string, map[string]string) bool { return false }

// isPinnedToolMessage 检查工具消息是否被 pin checker 钉住，被钉住的消息不参与微压缩。
func isPinnedToolMessage(toolName string, metadata map[string]string, checker MicroCompactPinChecker) bool {
	if checker == nil || len(metadata) == 0 {
		return false
	}
	return checker.ShouldPin(toolName, metadata)
}

// toolNameFromCallID 在 toolNames 映射中查找 callID 对应的工具名。
func toolNameFromCallID(callID string, toolNames map[string]string) string {
	return toolNames[strings.TrimSpace(callID)]
}
