package service

import (
	"path/filepath"
	"regexp"
	"strings"

	"go-llm-demo/internal/server/domain"

	"github.com/bmatcuk/doublestar/v4"
)

// SecurityService 根据工具类型和目标对象给出允许、拒绝或询问的决策。
type SecurityService interface {
	Check(toolType string, target string) domain.Action
}

// securityServiceImpl 是 SecurityService 的具体实现
type securityServiceImpl struct {
	configRepo domain.SecurityConfigRepository
	blackList  *domain.Config
	whiteList  *domain.Config
	yellowList *domain.Config
}

// NewSecurityService 创建基于规则配置仓库的安全检查服务。
func NewSecurityService(configRepo domain.SecurityConfigRepository) SecurityService {
	return &securityServiceImpl{
		configRepo: configRepo,
	}
}

func (s *securityServiceImpl) Initialize(configDir string) error {
	blackList, whiteList, yellowList, err := s.configRepo.LoadAll(configDir)
	if err != nil {
		return err
	}
	s.blackList = blackList
	s.whiteList = whiteList
	s.yellowList = yellowList
	return nil
}

func (s *securityServiceImpl) Check(toolType string, target string) domain.Action {
	normalizedTarget := target

	// 安全增强：对路径类操作进行规范化处理
	if toolType == "Read" || toolType == "Write" {
		// 1. 清洗路径 (消除 ../, ./, /// 等绕过风险)
		// 2. 统一斜杠 (Windows \ 转为 /) 以匹配 doublestar 规范
		normalizedTarget = filepath.ToSlash(filepath.Clean(target))

		// 3. 边界防御：如果路径尝试跳出当前工作目录（以 .. 开头）
		// 这通常是恶意的路径穿越尝试，直接拒绝。
		if strings.HasPrefix(normalizedTarget, "..") {
			return domain.ActionDeny
		}
	}

	// 业务规则 1：黑名单优先级最高（直接拒绝）
	if s.matchesList(s.blackList, toolType, normalizedTarget) {
		return domain.ActionDeny
	}

	// 业务规则 2：白名单次之（直接允许）
	if s.matchesList(s.whiteList, toolType, normalizedTarget) {
		return domain.ActionAllow
	}

	// 业务规则 3：黄名单再次之（询问用户）
	if s.matchesList(s.yellowList, toolType, normalizedTarget) {
		return domain.ActionAsk
	}

	// 兜底策略：默认询问用户
	return domain.ActionAsk
}

func (s *securityServiceImpl) matchesList(config *domain.Config, toolType, target string) bool {
	if config == nil {
		return false
	}

	for _, rule := range config.Rules {
		if ruleMatches(rule, toolType, target) {
			return true
		}
	}
	return false
}

func ruleMatches(rule domain.Rule, toolType string, target string) bool {
	var pattern string
	var actionBit string

	switch toolType {
	case "Read":
		pattern = rule.Target
		actionBit = rule.Read
	case "Write":
		pattern = rule.Target
		actionBit = rule.Write
	case "Bash":
		pattern = rule.Command
		actionBit = rule.Exec
	case "WebFetch":
		pattern = rule.Domain
		actionBit = rule.Network
	default:
		return false
	}

	// 必须同时具备匹配模式和对应的权限位设置，防止规则短路
	if pattern == "" || actionBit == "" {
		return false
	}

	// Bash 规则匹配命令字符串；其余规则统一走 doublestar 路径/域名匹配。
	if toolType == "Bash" {
		return matchCommand(pattern, target)
	}

	matched, err := doublestar.Match(pattern, target)
	if err != nil {
		return false
	}

	return matched
}

func matchCommand(pattern, command string) bool {
	// 命令规则沿用类似 glob 的写法，再转换为正则以便复用配置表达力。
	rePattern := regexp.QuoteMeta(pattern)
	rePattern = strings.ReplaceAll(rePattern, `\*\*`, `.*`)
	rePattern = strings.ReplaceAll(rePattern, `\*`, `.*`)
	re, err := regexp.Compile("^" + rePattern + "$")
	if err != nil {
		return false
	}
	return re.MatchString(command)
}
