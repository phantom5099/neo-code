package tool

import (
	"fmt"
	"log"
	"strings"
	"sync"

	toolsecurity "neo-code/internal/tool/security"
)

type SecurityChecker = toolsecurity.Checker
type Action = toolsecurity.Action
type Rule = toolsecurity.Rule
type Config = toolsecurity.Config

const (
	ActionDeny  = toolsecurity.ActionDeny
	ActionAllow = toolsecurity.ActionAllow
	ActionAsk   = toolsecurity.ActionAsk
)

var (
	securityCheckerMu sync.RWMutex
	securityChecker   SecurityChecker

	securityAskApprovalMu sync.Mutex
	securityAskApprovals  = map[string]int{}
)

func SetSecurityChecker(checker SecurityChecker) {
	securityCheckerMu.Lock()
	securityChecker = checker
	securityCheckerMu.Unlock()
}

func InitializeSecurity(configDir string) error {
	policy, err := toolsecurity.LoadPolicy(configDir)
	if err != nil {
		return err
	}
	SetSecurityChecker(policy)
	return nil
}

func getSecurityChecker() SecurityChecker {
	securityCheckerMu.RLock()
	checker := securityChecker
	securityCheckerMu.RUnlock()
	return checker
}

func ApproveSecurityAsk(toolType, target string) {
	key, err := securityApprovalKey(toolType, target)
	if err != nil {
		log.Printf("warning: failed to record security ask approval: %v", err)
		return
	}
	securityAskApprovalMu.Lock()
	securityAskApprovals[key]++
	securityAskApprovalMu.Unlock()
}

func consumeSecurityAskApproval(toolType, target string) bool {
	key, err := securityApprovalKey(toolType, target)
	if err != nil {
		log.Printf("warning: failed to consume security ask approval: %v", err)
		return false
	}
	securityAskApprovalMu.Lock()
	defer securityAskApprovalMu.Unlock()
	count := securityAskApprovals[key]
	if count <= 0 {
		return false
	}
	if count == 1 {
		delete(securityAskApprovals, key)
		return true
	}
	securityAskApprovals[key] = count - 1
	return true
}

func securityApprovalKey(toolType, target string) (string, error) {
	normalizedType := strings.ToLower(strings.TrimSpace(toolType))
	normalizedTarget := strings.TrimSpace(target)
	if normalizedType == "" || normalizedTarget == "" {
		return "", fmt.Errorf("invalid security approval context: toolType=%q target=%q", toolType, target)
	}
	return normalizedType + "\x00" + normalizedTarget, nil
}

func guardToolExecution(toolType, target, toolName string) *ToolResult {
	checker := getSecurityChecker()
	if checker == nil {
		return nil
	}

	action := checker.Check(toolType, target)
	metadata := map[string]interface{}{
		"securityToolType": toolType,
		"securityTarget":   target,
		"securityAction":   string(action),
	}

	switch action {
	case ActionAllow:
		return nil
	case ActionDeny:
		return &ToolResult{
			ToolName: toolName,
			Success:  false,
			Error:    fmt.Sprintf("Security policy denied execution of %s: %s", toolType, target),
			Metadata: metadata,
		}
	case ActionAsk:
		if consumeSecurityAskApproval(toolType, target) {
			return nil
		}
		return &ToolResult{
			ToolName: toolName,
			Success:  false,
			Error:    fmt.Sprintf("Execution of %s on %s requires user confirmation (Action: Ask).", toolType, target),
			Metadata: metadata,
		}
	default:
		return nil
	}
}

func GuardToolExecution(toolType, target, toolName string) *ToolResult {
	return guardToolExecution(toolType, target, toolName)
}
