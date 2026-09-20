package ace

import "testing"

func TestDiagnoseBottleneckCompute(t *testing.T) {
	d := DiagnoseBottleneck(FailureTelemetry{
		TaskID: "compute",
		BudgetTrials: []BudgetTrial{
			{Multiplier: 1, Solved: false},
			{Multiplier: 2, Solved: true},
		},
	})
	if d.Kind != BottleneckCompute {
		t.Fatalf("got %q want %q", d.Kind, BottleneckCompute)
	}
}

func TestDiagnoseBottleneckSearch(t *testing.T) {
	d := DiagnoseBottleneck(FailureTelemetry{
		TaskID: "search",
		SearchExhausted: true,
		SearchOrdersTested: 1,
		AlternateOrderSolved: true,
	})
	if d.Kind != BottleneckSearch {
		t.Fatalf("got %q want %q", d.Kind, BottleneckSearch)
	}
}

func TestDiagnoseBottleneckRepresentation(t *testing.T) {
	d := DiagnoseBottleneck(FailureTelemetry{
		TaskID: "representation",
		SearchExhausted: true,
		SearchOrdersTested: 3,
		AllCandidateFamiliesExhausted: true,
		RepresentationAudit: RepresentationAudit{
			CurrentFamily: "arithmetic-expression",
			RequiredFeatures: []string{"state-transition"},
			SupportedFeatures: []string{"constant", "arithmetic"},
		},
	})
	if d.Kind != BottleneckRepresentation {
		t.Fatalf("got %q want %q", d.Kind, BottleneckRepresentation)
	}
}

func TestDiagnoseBottleneckInsufficientEvidenceDoesNotInventDiagnosis(t *testing.T) {
	d := DiagnoseBottleneck(FailureTelemetry{
		TaskID: "ambiguous",
		SearchExhausted: true,
		SearchOrdersTested: 1,
		AllCandidateFamiliesExhausted: true,
	})
	if d.Kind != BottleneckInsufficient {
		t.Fatalf("got %q want %q", d.Kind, BottleneckInsufficient)
	}
}

func TestDiagnoseBottleneckVerification(t *testing.T) {
	d := DiagnoseBottleneck(FailureTelemetry{
		TaskID: "verification",
		CandidateReachedVerifier: true,
		IndependentVerifierRejected: true,
		EvidenceCount: 2,
		MinimumEvidence: 1,
	})
	if d.Kind != BottleneckVerification {
		t.Fatalf("got %q want %q", d.Kind, BottleneckVerification)
	}
}
