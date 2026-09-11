package ace

import "sort"

// FrontierFailureEvidence is evaluator-neutral telemetry emitted after an
// attempted task. It contains observations of the failure, not an answer for
// what capability should be acquired.
type FrontierFailureEvidence struct {
	TaskID              string
	Regime              FrontierRegime
	Attempted            bool
	ExecutionSucceeded   bool
	PlanProduced         bool
	VerificationFailed   bool
	PredictionError      float64
	NovelStateCount      int
	ExperimentCount      int
	SearchExhausted      bool
	RepresentationLoss   bool
	MemoryHit            bool
	CandidateCount       int
	CandidateFailures    int
}

type GapHypothesis struct {
	ID          string
	Class       string
	Statement   string
	EvidenceIDs []string
	Score       float64
	Test        string
}

type CapabilityGapReport struct {
	TaskID       string
	Primary      GapHypothesis
	Alternatives []GapHypothesis
	Objective    CapabilitySpecification
}

// DiagnoseFrontierGap ranks explanations from observable telemetry. It never
// accepts a benchmark-author supplied capability name and always preserves
// at least two alternatives when the evidence supports them.
func DiagnoseFrontierGap(e FrontierFailureEvidence) CapabilityGapReport {
	var hs []GapHypothesis
	add := func(class, statement, test string, score float64) {
		hs = append(hs, GapHypothesis{
			ID: Hash([]any{"gap", e.TaskID, class}), Class: class,
			Statement: statement, EvidenceIDs: []string{e.TaskID}, Score: score, Test: test,
		})
	}
	if e.RepresentationLoss || e.NovelStateCount > 0 && e.PredictionError > 0.5 {
		add("representation", "observable task state is insufficient for the attempted prediction", "retain observations with an alternate state encoding", 3)
	}
	if e.SearchExhausted || e.CandidateCount == 0 {
		add("search", "available acquisition/search substrate did not produce a viable candidate", "repeat with the same evidence and an independent search budget", 2.5)
	}
	if e.ExperimentCount == 0 && e.PredictionError > 0 {
		add("causal-model", "the system has unresolved predictive uncertainty and has not experimentally discriminated it", "run an intervention chosen for expected uncertainty reduction", 2)
	}
	if !e.PlanProduced || e.CandidateFailures > 0 {
		add("planning", "the current planner failed to construct a verified route to the goal", "hold representation fixed and vary planning only", 1.5)
	}
	if e.VerificationFailed {
		add("verification", "a proposed action failed independent verification", "replay proposal with an independent verifier", 1)
	}
	if len(hs) == 0 {
		add("execution", "the observed failure is not yet localized to a specific capability layer", "repeat with layer-isolating telemetry", 0.5)
	}
	sort.SliceStable(hs, func(i, j int) bool {
		if hs[i].Score != hs[j].Score { return hs[i].Score > hs[j].Score }
		return hs[i].Class < hs[j].Class
	})
	primary := hs[0]
	alternatives := append([]GapHypothesis(nil), hs[1:]...)
	objective := CapabilitySpecification{
		ID: Hash([]any{"gap-objective", e.TaskID, primary.ID}),
		DesiredBehaviour: primary.Statement,
		Inputs: []string{"observable failure evidence"},
		Outputs: []string{"verified capability change"},
		Invariants: []string{"no evaluator-provided capability label", "independent verification"},
		AcceptanceTests: []string{"held-out generated task", "regression on prior frontier"},
		ResourceLimits: ResourceVector{Compute: 50, Memory: 50, Storage: 5, TimeMS: 1000, ExperimentBudget: 10},
		FailureCriteria: []string{"no causal improvement", "regression", "verification failure"},
		RegressionConstraints: []string{"existing frontier remains solvable"},
		Provenance: Prov("capability-gap-diagnosis", e.TaskID, primary.Class, e),
	}
	return CapabilityGapReport{TaskID: e.TaskID, Primary: primary, Alternatives: alternatives, Objective: objective}
}
