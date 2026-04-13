package approval

// Decision 表示审批请求的最终处理结果。
type Decision string

const (
	DecisionAllowOnce    Decision = "allow_once"
	DecisionAllowSession Decision = "allow_session"
	DecisionReject       Decision = "reject"
)
