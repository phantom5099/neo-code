package security

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type Policy struct {
	blacklist  *Config
	whitelist  *Config
	yellowlist *Config
}

func LoadPolicy(configDir string) (*Policy, error) {
	loader := ConfigLoader{}
	blacklist, whitelist, yellowlist, err := loader.LoadDir(configDir)
	if err != nil {
		return nil, err
	}
	return &Policy{
		blacklist:  blacklist,
		whitelist:  whitelist,
		yellowlist: yellowlist,
	}, nil
}

func (p *Policy) Check(toolType string, target string) Action {
	normalizedType := normalizeToolType(toolType)
	normalizedTarget := strings.TrimSpace(target)

	if normalizedType == "Read" || normalizedType == "Write" {
		normalizedTarget = filepath.ToSlash(filepath.Clean(target))
		if strings.HasPrefix(normalizedTarget, "..") {
			return ActionDeny
		}
	}

	if matchesConfig(p.blacklist, normalizedType, normalizedTarget) {
		return ActionDeny
	}
	if matchesConfig(p.whitelist, normalizedType, normalizedTarget) {
		return ActionAllow
	}
	if matchesConfig(p.yellowlist, normalizedType, normalizedTarget) {
		return ActionAsk
	}

	return ActionAsk
}

func matchesConfig(config *Config, toolType, target string) bool {
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

func ruleMatches(rule Rule, toolType string, target string) bool {
	var pattern string
	var actionBit string

	switch normalizeToolType(toolType) {
	case "Read":
		pattern = rule.Target
		actionBit = rule.Read
	case "Write":
		pattern = rule.Target
		actionBit = rule.Write
	case "Bash":
		pattern = rule.Command
		actionBit = rule.Exec
	case "Webfetch":
		pattern = rule.Domain
		actionBit = rule.Network
	default:
		return false
	}

	if strings.TrimSpace(pattern) == "" || strings.TrimSpace(actionBit) == "" {
		return false
	}

	if normalizeToolType(toolType) == "Bash" {
		return matchCommand(pattern, target)
	}

	matched, err := doublestar.Match(pattern, target)
	if err != nil {
		return false
	}
	return matched
}

func matchCommand(pattern, command string) bool {
	rePattern := regexp.QuoteMeta(pattern)
	rePattern = strings.ReplaceAll(rePattern, `\*\*`, `.*`)
	rePattern = strings.ReplaceAll(rePattern, `\*`, `.*`)

	re, err := regexp.Compile("^" + rePattern + "$")
	if err != nil {
		return false
	}
	return re.MatchString(command)
}

func normalizeToolType(toolType string) string {
	switch strings.ToLower(strings.TrimSpace(toolType)) {
	case "read":
		return "Read"
	case "write":
		return "Write"
	case "edit":
		return "Edit"
	case "bash":
		return "Bash"
	case "list":
		return "List"
	case "grep":
		return "Grep"
	case "webfetch", "web_fetch":
		return "Webfetch"
	case "websearch", "web_search":
		return "Webfetch"
	case "todo":
		return "Todo"
	default:
		return strings.TrimSpace(toolType)
	}
}
