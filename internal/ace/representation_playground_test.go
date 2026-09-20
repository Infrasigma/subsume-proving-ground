package ace

import (
	"context"
	"testing"
)

func TestRepresentationPlaygroundCounterfactualFindsVerifiedDerivedFeature(t *testing.T) {
	train := []ProgramTestCase{
		{Input: map[string]string{"x": "-5"}, Expected: map[string]string{"y": "5"}},
		{Input: map[string]string{"x": "-1"}, Expected: map[string]string{"y": "2"}},
		{Input: map[string]string{"x": "0"}, Expected: map[string]string{"y": "2"}},
		{Input: map[string]string{"x": "3"}, Expected: map[string]string{"y": "3"}},
		{Input: map[string]string{"x": "5"}, Expected: map[string]string{"y": "5"}},
	}
	hidden := []ProgramTestCase{
		{Input: map[string]string{"x": "-8"}, Expected: map[string]string{"y": "8"}},
		{Input: map[string]string{"x": "1"}, Expected: map[string]string{"y": "2"}},
		{Input: map[string]string{"x": "9"}, Expected: map[string]string{"y": "9"}},
	}

	spec, err := GeneralCapabilitySpecification(
		Task{ID: "repr-playground", Goal: "y equals max(abs(x),2)"},
		train,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Establish the baseline failure using the same builder that the
	// counterfactual runner will use. This keeps the representation gate
	// downstream of the existing executable substrate rather than inventing
	// a separate baseline.
	if proposal, buildErr := (UniversalProgramBuilder{}).Build(
		ArchitectureCandidate{
			Mechanism: "universal:branching",
			Interfaces: []string{"typed-key-value-input", "executable-program"},
			Resources:  spec.ResourceLimits,
		},
		spec,
	); buildErr == nil {
		var p UniversalProgram
		if jsonErr := unmarshalJSON([]byte(proposal.Artifact), &p); jsonErr == nil && programFits(p, hidden) {
			t.Fatal("baseline scalar representation unexpectedly solved the held-out target")
		}
	}

	state := RepresentationPlaygroundState{
		Spec:   spec,
		Hidden: hidden,
		Blocks: DefaultRepresentationBlocks(spec),
	}
	opaque, err := EncodeRepresentationPlaygroundState(state)
	if err != nil {
		t.Fatal(err)
	}

	failure := FailureTelemetry{
		TaskID:                         spec.ID,
		EvidenceCount:                  len(train),
		MinimumEvidence:               1,
		SearchExhausted:                true,
		AllCandidateFamiliesExhausted: true,
		CurrentRepresentation:          "scalar-expression",
		SupportedRepresentationFeatures: []string{"scalar"},
		RequiredRepresentationFeatures: []string{"derived-feature"},
	}

	out, err := RunDiscriminatingBottleneckExperiments(
		context.Background(),
		failure,
		opaque,
		RepresentationPlaygroundRunner{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Discriminated {
		t.Fatalf("expected unique representation diagnosis, got %#v", out.Diagnosis)
	}
	if out.Diagnosis.Class != BottleneckRepresentation {
		t.Fatalf("got %q want %q", out.Diagnosis.Class, BottleneckRepresentation)
	}

	var representationTrial *CounterfactualTrial
	for i := range out.Trials {
		if out.Trials[i].Hypothesis == HypothesisRepresentation {
			representationTrial = &out.Trials[i]
			break
		}
	}
	if representationTrial == nil {
		t.Fatal("representation trial missing")
	}
	if !representationTrial.Result.Solved || !representationTrial.Result.IndependentlyVerified {
		t.Fatalf("representation intervention was not independently verified: %#v", representationTrial.Result)
	}
	if len(representationTrial.Result.Evidence) < 4 {
		t.Fatalf("representation trial did not record semantic delta evidence: %#v", representationTrial.Result.Evidence)
	}
}
