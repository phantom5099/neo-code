package controlplane

import "testing"

func TestDecideTurnBudgetAccurateBranches(t *testing.T) {
	t.Parallel()

	baseEstimate := TurnBudgetEstimate{
		ID: TurnBudgetID{
			AttemptSeq:  1,
			RequestHash: "hash-1",
		},
		EstimatedInputTokens: 120,
		EstimateSource:       "provider",
		Accurate:             true,
	}

	within := DecideTurnBudget(baseEstimate, 120, 0)
	if within.Action != TurnBudgetActionAllow {
		t.Fatalf("within.Action = %q", within.Action)
	}
	if within.Reason != BudgetDecisionReasonWithinBudget {
		t.Fatalf("within.Reason = %q", within.Reason)
	}
	if !within.EstimateAccurate {
		t.Fatalf("within.EstimateAccurate = false, want true")
	}

	firstExceed := DecideTurnBudget(baseEstimate, 100, 0)
	if firstExceed.Action != TurnBudgetActionCompact {
		t.Fatalf("firstExceed.Action = %q", firstExceed.Action)
	}
	if firstExceed.Reason != BudgetDecisionReasonExceedsBudgetFirstTime {
		t.Fatalf("firstExceed.Reason = %q", firstExceed.Reason)
	}
	if !firstExceed.EstimateAccurate {
		t.Fatalf("firstExceed.EstimateAccurate = false, want true")
	}

	afterCompact := DecideTurnBudget(baseEstimate, 100, 1)
	if afterCompact.Action != TurnBudgetActionStop {
		t.Fatalf("afterCompact.Action = %q", afterCompact.Action)
	}
	if afterCompact.Reason != BudgetDecisionReasonExceedsBudgetAfterCompact {
		t.Fatalf("afterCompact.Reason = %q", afterCompact.Reason)
	}
	if !afterCompact.EstimateAccurate {
		t.Fatalf("afterCompact.EstimateAccurate = false, want true")
	}
}

func TestDecideTurnBudgetInaccurateBranches(t *testing.T) {
	t.Parallel()

	estimate := TurnBudgetEstimate{
		ID: TurnBudgetID{
			AttemptSeq:  2,
			RequestHash: "hash-2",
		},
		EstimatedInputTokens: 200,
		EstimateSource:       "local",
		Accurate:             false,
	}

	firstExceed := DecideTurnBudget(estimate, 100, 0)
	if firstExceed.Action != TurnBudgetActionCompact {
		t.Fatalf("firstExceed.Action = %q", firstExceed.Action)
	}
	if firstExceed.Reason != BudgetDecisionReasonExceedsBudgetInaccurateFirstTime {
		t.Fatalf("firstExceed.Reason = %q", firstExceed.Reason)
	}
	if firstExceed.EstimateAccurate {
		t.Fatalf("firstExceed.EstimateAccurate = true, want false")
	}

	afterCompact := DecideTurnBudget(estimate, 100, 1)
	if afterCompact.Action != TurnBudgetActionAllow {
		t.Fatalf("afterCompact.Action = %q", afterCompact.Action)
	}
	if afterCompact.Reason != BudgetDecisionReasonExceedsBudgetInaccurateAfterCompactAllow {
		t.Fatalf("afterCompact.Reason = %q", afterCompact.Reason)
	}
	if afterCompact.EstimateAccurate {
		t.Fatalf("afterCompact.EstimateAccurate = true, want false")
	}
}
