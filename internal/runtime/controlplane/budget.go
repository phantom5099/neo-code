package controlplane

// TurnBudgetAction 表示预算控制面对单次发送尝试做出的唯一动作。
type TurnBudgetAction string

const (
	TurnBudgetActionAllow   TurnBudgetAction = "allow"
	TurnBudgetActionCompact TurnBudgetAction = "compact"
	TurnBudgetActionStop    TurnBudgetAction = "stop"
)

const (
	// BudgetDecisionReasonWithinBudget 表示估算在预算范围内。
	BudgetDecisionReasonWithinBudget = "within_budget"
	// BudgetDecisionReasonExceedsBudgetFirstTime 表示首次超预算，需要先 compact。
	BudgetDecisionReasonExceedsBudgetFirstTime = "exceeds_budget_first_time"
	// BudgetDecisionReasonExceedsBudgetAfterCompact 表示高置信估算在 compact 后仍超预算，需要停止。
	BudgetDecisionReasonExceedsBudgetAfterCompact = "exceeds_budget_after_compact"
	// BudgetDecisionReasonExceedsBudgetInaccurateFirstTime 表示低置信估算首次超预算，先 compact 再验证。
	BudgetDecisionReasonExceedsBudgetInaccurateFirstTime = "exceeds_budget_inaccurate_first_time"
	// BudgetDecisionReasonExceedsBudgetInaccurateAfterCompactAllow 表示低置信估算 compact 后仍超预算但允许放行。
	BudgetDecisionReasonExceedsBudgetInaccurateAfterCompactAllow = "exceeds_budget_inaccurate_after_compact_allow"
)

// TurnBudgetID 标识一次冻结预算尝试，避免 estimate、decision 与 usage observation 串用。
type TurnBudgetID struct {
	AttemptSeq  int    `json:"attempt_seq"`
	RequestHash string `json:"request_hash"`
}

// TurnBudgetEstimate 描述 runtime 对冻结请求输入 token 的主干估算事实。
type TurnBudgetEstimate struct {
	ID                   TurnBudgetID `json:"id"`
	EstimatedInputTokens int          `json:"estimated_input_tokens"`
	EstimateSource       string       `json:"estimate_source,omitempty"`
	Accurate             bool         `json:"accurate"`
}

// TurnBudgetDecision 描述冻结请求在当前预算事实下的决策结果。
type TurnBudgetDecision struct {
	ID                   TurnBudgetID     `json:"id"`
	Action               TurnBudgetAction `json:"action"`
	Reason               string           `json:"reason,omitempty"`
	EstimatedInputTokens int              `json:"estimated_input_tokens"`
	PromptBudget         int              `json:"prompt_budget"`
	EstimateSource       string           `json:"estimate_source,omitempty"`
	EstimateAccurate     bool             `json:"estimate_accurate"`
}

// DecideTurnBudget 根据输入预算事实输出 allow、compact 或 stop 三种动作。
func DecideTurnBudget(
	estimate TurnBudgetEstimate,
	promptBudget int,
	compactCount int,
) TurnBudgetDecision {
	decision := TurnBudgetDecision{
		ID:                   estimate.ID,
		EstimatedInputTokens: estimate.EstimatedInputTokens,
		PromptBudget:         promptBudget,
		EstimateSource:       estimate.EstimateSource,
		EstimateAccurate:     estimate.Accurate,
	}
	if estimate.EstimatedInputTokens <= promptBudget {
		decision.Action = TurnBudgetActionAllow
		decision.Reason = BudgetDecisionReasonWithinBudget
		return decision
	}
	if compactCount == 0 {
		decision.Action = TurnBudgetActionCompact
		if estimate.Accurate {
			decision.Reason = BudgetDecisionReasonExceedsBudgetFirstTime
		} else {
			decision.Reason = BudgetDecisionReasonExceedsBudgetInaccurateFirstTime
		}
		return decision
	}
	if estimate.Accurate {
		decision.Action = TurnBudgetActionStop
		decision.Reason = BudgetDecisionReasonExceedsBudgetAfterCompact
		return decision
	}
	decision.Action = TurnBudgetActionAllow
	decision.Reason = BudgetDecisionReasonExceedsBudgetInaccurateAfterCompactAllow
	return decision
}
