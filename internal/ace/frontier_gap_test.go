package ace

import "testing"

func TestDiagnoseFrontierGapRequiresEvidenceDerivedAlternatives(t *testing.T) {
	e := FrontierFailureEvidence{
		TaskID: "task-1", Regime: RegimeInteractive,
		Attempted: true, ExecutionSucceeded: true, PlanProduced: false,
		VerificationFailed: false, PredictionError: 1, NovelStateCount: 2,
		ExperimentCount: 0, SearchExhausted: true, RepresentationLoss: true,
		CandidateCount: 0, CandidateFailures: 2,
	}
	r := DiagnoseFrontierGap(e)
	if r.Primary.Class == "" || len(r.Alternatives) < 2 {
		t.Fatalf("expected competing gap explanations, got primary=%q alternatives=%d", r.Primary.Class, len(r.Alternatives))
	}
	for _, h := range append([]GapHypothesis{r.Primary}, r.Alternatives...) {
		if len(h.EvidenceIDs) != 1 || h.EvidenceIDs[0] != e.TaskID {
			t.Fatalf("gap %s is not grounded in task evidence: %#v", h.Class, h.EvidenceIDs)
		}
		if h.Test == "" {
			t.Fatalf("gap %s has no discriminating test", h.Class)
		}
	}
	if r.Objective.ID == "" || len(r.Objective.AcceptanceTests) != 2 {
		t.Fatal("missing testable capability-acquisition objective")
	}
}

func TestDiagnoseFrontierGapDoesNotInventSpecificCapabilityName(t *testing.T) {
	r := DiagnoseFrontierGap(FrontierFailureEvidence{TaskID: "task-2", Attempted: true})
	if r.Primary.Statement == "" || r.Primary.Class == "" {
		t.Fatal("unlocalized failure must still produce an explicit uncertainty statement")
	}
	if r.Primary.Class == "answer" || r.Primary.Class == "benchmark" {
		t.Fatalf("diagnosis leaked evaluator semantics: %q", r.Primary.Class)
	}
}
