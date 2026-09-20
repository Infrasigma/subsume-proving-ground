package ace

import "testing"

func TestDiagnoseFailureTelemetryComputeBoundary(t *testing.T) {
	d := DiagnoseFailureTelemetry(FailureTelemetry{
		TaskID: "compute",
		EvidenceCount: 2,
		MinimumEvidence: 1,
		BudgetTrials: []BudgetTrial{
			{Multiplier: 1, Solved: false},
			{Multiplier: 2, Solved: true},
		},
	})
	if d.Class != BottleneckSearchSpace {
		t.Fatalf("got %q want %q", d.Class, BottleneckSearchSpace)
	}
}

func TestDiagnoseFailureTelemetrySearchOrder(t *testing.T) {
	d := DiagnoseFailureTelemetry(FailureTelemetry{
		TaskID: "search",
		EvidenceCount: 2,
		MinimumEvidence: 1,
		SearchExhausted: true,
		SearchOrdersTested: 1,
		AlternateOrderSolved: true,
	})
	if d.Class != BottleneckSearchOrder {
		t.Fatalf("got %q want %q", d.Class, BottleneckSearchOrder)
	}
}

func TestDiagnoseFailureTelemetryRepresentation(t *testing.T) {
	d := DiagnoseFailureTelemetry(FailureTelemetry{
		TaskID: "representation",
		EvidenceCount: 4,
		MinimumEvidence: 1,
		SearchExhausted: true,
		SearchOrdersTested: 3,
		AllCandidateFamiliesExhausted: true,
		CurrentRepresentation: "arithmetic-expression",
		RequiredRepresentationFeatures: []string{"state-transition"},
		SupportedRepresentationFeatures: []string{"constant", "arithmetic"},
	})
	if d.Class != BottleneckRepresentation {
		t.Fatalf("got %q want %q", d.Class, BottleneckRepresentation)
	}
}

func TestDiagnoseFailureTelemetryDoesNotInventDiagnosis(t *testing.T) {
	d := DiagnoseFailureTelemetry(FailureTelemetry{
		TaskID: "ambiguous",
		EvidenceCount: 2,
		MinimumEvidence: 1,
		SearchExhausted: true,
		SearchOrdersTested: 1,
		AllCandidateFamiliesExhausted: true,
	})
	if d.Class != BottleneckUnknown {
		t.Fatalf("got %q want %q", d.Class, BottleneckUnknown)
	}
}

func TestDiagnoseFailureTelemetryVerification(t *testing.T) {
	d := DiagnoseFailureTelemetry(FailureTelemetry{
		TaskID: "verification",
		CandidateReachedVerifier: true,
		IndependentVerifierRejected: true,
		EvidenceCount: 2,
		MinimumEvidence: 1,
	})
	if d.Class != BottleneckVerification {
		t.Fatalf("got %q want %q", d.Class, BottleneckVerification)
	}
}

func TestProjectAcquisitionTelemetryIsExplicitAndLossy(t *testing.T) {
	tm := AcquisitionTelemetry{
		TaskID: "projection",
		KnownExamples: 3,
		CandidateCount: 2,
		CandidateFailures: []string{"candidate-a", "candidate-b"},
		Representation: []string{"arithmetic-expression"},
		VerificationOutcomes: []string{"independent verifier: reject"},
	}
	f := ProjectAcquisitionTelemetry(tm)
	if !f.SearchExhausted || !f.AllCandidateFamiliesExhausted {
		t.Fatal("expected exhausted search frontier in projected telemetry")
	}
	if f.CurrentRepresentation != "arithmetic-expression" {
		t.Fatalf("unexpected representation projection: %q", f.CurrentRepresentation)
	}
	if !f.IndependentVerifierRejected {
		t.Fatal("expected verifier rejection to survive projection")
	}
	// No budget/resource fact was present in AcquisitionTelemetry; the projection
	// must not manufacture a compute diagnosis.
	if len(f.BudgetTrials) != 0 {
		t.Fatal("projection manufactured resource trials")
	}
}

func TestDiagnoseAdaptiveBoundaryFallsBackWithoutInventing(t *testing.T) {
	d := DiagnoseAdaptiveBoundary(AcquisitionTelemetry{
		TaskID: "ambiguous",
		KnownExamples: 3,
		CandidateCount: 2,
		CandidateFailures: []string{"a"},
		Representation: []string{"arithmetic-expression"},
		VerificationOutcomes: []string{"pending"},
	})
	if d.Class == BottleneckRepresentation {
		t.Fatal("ambiguous telemetry was incorrectly promoted to representation diagnosis")
	}
}
