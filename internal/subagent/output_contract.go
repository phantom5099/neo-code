package subagent

import "strings"

var supportedOutputSections = map[string]struct{}{
	"summary":      {},
	"findings":     {},
	"patches":      {},
	"risks":        {},
	"next_actions": {},
	"artifacts":    {},
}

// validateOutputContract 校验输出结构是否满足角色策略要求。
func validateOutputContract(policy RolePolicy, output Output) error {
	out := output.normalize()
	for _, section := range dedupeAndTrim(policy.RequiredSections) {
		key := strings.ToLower(strings.TrimSpace(section))
		if _, ok := supportedOutputSections[key]; !ok {
			return errorsf("unsupported required output section %q", section)
		}
		if key == "summary" && strings.TrimSpace(out.Summary) == "" {
			return errorsf("output summary is required")
		}
	}
	return nil
}
