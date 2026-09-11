package ace

import (
	"encoding/json"
	"fmt"
)

func (p aoProposal) MarshalJSON() ([]byte, error) {
	baselineEquivalent := p.Terminal == "REJECTED_SEMANTICS"
	novelty := !baselineEquivalent && p.Terminal == "ACCEPTED"
	verification := p.Terminal == "REJECTED_SEMANTICS" || p.Terminal == "ACCEPTED"
	executionStatus := "not_executed"
	if p.Terminal == "REJECTED_GENERATION" {
		executionStatus = "generation_failed"
	}
	type evidence struct {
		ProposalID string `json:"proposal_id"`
		CandidateOperatorRepresentation string `json:"candidate_operator_representation"`
		CandidateSource string `json:"candidate_source"`
		ProcedureSteps interface{} `json:"procedure_steps"`
		ProcedureUnavailableReason string `json:"procedure_steps_unavailable_reason,omitempty"`
		ExecutableSemanticDescription string `json:"executable_semantic_description"`
		BehavioralProbeInputs interface{} `json:"behavioral_probe_inputs"`
		BehavioralProbeInputsUnavailableReason string `json:"behavioral_probe_inputs_unavailable_reason,omitempty"`
		BehavioralProbeOutputs interface{} `json:"behavioral_probe_outputs"`
		BehavioralProbeOutputsUnavailableReason string `json:"behavioral_probe_outputs_unavailable_reason,omitempty"`
		BehavioralSignature string `json:"behavioral_signature"`
		BaselineEquivalent bool `json:"baseline_equivalent"`
		NoveltyResult bool `json:"novelty_result"`
		ExecutionStatus string `json:"execution_status"`
		IndependentVerificationStatus string `json:"independent_verification_status"`
		VerificationResult bool `json:"verification_result"`
		TerminalClassification string `json:"terminal_classification"`
		RejectionReason string `json:"rejection_reason,omitempty"`
		ResourceBudget map[string]int `json:"resource_budget"`
	}
	return json.Marshal(evidence{
		ProposalID: p.ID,
		CandidateOperatorRepresentation: p.Representation,
		CandidateSource: "frozen-baseline-acquisition-language",
		ProcedureSteps: nil,
		ProcedureUnavailableReason: "runtime proposal record retains executable representation but not decomposed builder procedure steps",
		ExecutableSemanticDescription: p.Semantics,
		BehavioralProbeInputs: nil,
		BehavioralProbeInputsUnavailableReason: "runtime proposal record retains only the derived behavioral signature, not the probe vector",
		BehavioralProbeOutputs: nil,
		BehavioralProbeOutputsUnavailableReason: "runtime proposal record retains only the derived behavioral signature, not per-probe outputs",
		BehavioralSignature: p.Signature,
		BaselineEquivalent: baselineEquivalent,
		NoveltyResult: novelty,
		ExecutionStatus: executionStatus,
		IndependentVerificationStatus: map[bool]string{true: "performed", false: "not_performed"}[verification],
		VerificationResult: verification,
		TerminalClassification: p.Terminal,
		RejectionReason: p.Reason,
		ResourceBudget: map[string]int{"proposal_budget": 20},
	})
}

func (c aoCondition) MarshalJSON() ([]byte, error) {
	type condition struct {
		Name string `json:"name"`
		Proposals []aoProposal `json:"proposals"`
		Installed string `json:"installed_operator"`
		FutureSearchExposed bool `json:"future_search_exposed"`
		HeldOut bool `json:"heldout_pass"`
		CandidateCount int `json:"candidate_count"`
		BudgetUsed int `json:"budget_used"`
	}
	if c.CandidateCount != len(c.Proposals) {
		return nil, fmt.Errorf("proposal evidence count mismatch for %s: runtime=%d serialized=%d", c.Name, c.CandidateCount, len(c.Proposals))
	}
	for i, p := range c.Proposals {
		if p.ID == "" || p.Terminal == "" {
			return nil, fmt.Errorf("proposal %d in %s lost proposal_id or terminal classification", i, c.Name)
		}
		if p.Terminal != "ACCEPTED" && p.Reason == "" {
			return nil, fmt.Errorf("proposal %d in %s rejected without rejection reason", i, c.Name)
		}
		if p.Signature == "" {
			return nil, fmt.Errorf("proposal %d in %s lost semantic novelty evidence", i, c.Name)
		}
		switch p.Terminal {
		case "REJECTED_GENERATION", "REJECTED_EXECUTION", "REJECTED_SEMANTICS", "REJECTED_VERIFICATION", "REJECTED_REGRESSION", "REJECTED_HELDOUT", "ACCEPTED":
		default:
			return nil, fmt.Errorf("proposal %d in %s has invalid terminal classification %q", i, c.Name, p.Terminal)
		}
	}
	return json.Marshal(condition{Name: c.Name, Proposals: c.Proposals, Installed: c.Installed, FutureSearchExposed: c.FutureSearchExposed, HeldOut: c.HeldOut, CandidateCount: c.CandidateCount, BudgetUsed: c.BudgetUsed})
}

func (r aoReport) MarshalJSON() ([]byte, error) {
	type plain aoReport
	return json.Marshal(plain(r))
}
