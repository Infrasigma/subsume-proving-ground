package ace

import (
	"context"
	"testing"
)

func TestCounterfactualRepresentationPromotesGeneratorAndRebalancesPolicy(t *testing.T) {
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
		Task{ID: "policy-promotion", Goal: "y equals max(abs(x),2)"},
		train,
	)
	if err != nil {
		t.Fatal(err)
	}

	failure := FailureTelemetry{
		TaskID:                            spec.ID,
		EvidenceCount:                     len(train),
		MinimumEvidence:                   1,
		SearchExhausted:                   true,
		AllCandidateFamiliesExhausted:     true,
		CurrentRepresentation:             "scalar-expression",
		SupportedRepresentationFeatures:    []string{"scalar"},
		RequiredRepresentationFeatures:    []string{"derived-feature"},
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

	counterfactual, err := RunDiscriminatingBottleneckExperiments(
		context.Background(),
		failure,
		opaque,
		RepresentationPlaygroundRunner{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !counterfactual.Discriminated || counterfactual.Diagnosis.Class != BottleneckRepresentation {
		t.Fatalf("expected uniquely discriminated representation failure: %#v", counterfactual.Diagnosis)
	}

	var policy AcquisitionPolicy
	mutation, err := PromoteCounterfactualRepresentation(&policy, failure, counterfactual)
	if err != nil {
		t.Fatal(err)
	}
	if mutation.PrimitiveID == "" || mutation.SearchWeight != 0.5 {
		t.Fatalf("unexpected policy mutation: %#v", mutation)
	}
	if len(policy.Primitives) != 1 || len(policy.Bindings) != 1 || len(policy.Mutations) != 1 {
		t.Fatalf("policy mutation did not persist: %#v", policy)
	}

	signature := FailureTopologySignature(failure)
	if got := policy.Weight(signature, mutation.PrimitiveID); got != 0.5 {
		t.Fatalf("got search weight %.2f want 0.50", got)
	}

	enriched, enrichedHidden, primitive, err := PrepareCapabilityWithAcquisitionPolicy(
		spec,
		hidden,
		failure,
		policy,
	)
	if err != nil {
		t.Fatal(err)
	}
	if primitive.ID != mutation.PrimitiveID {
		t.Fatalf("got primitive %q want %q", primitive.ID, mutation.PrimitiveID)
	}
	if len(enriched.Inputs) != len(spec.Inputs)+1 {
		t.Fatalf("generator space did not expand inputs: before=%v after=%v", spec.Inputs, enriched.Inputs)
	}

	builder := UniversalProgramBuilder{}
	candidates, err := (UniversalMechanismSearch{}).SearchMechanisms(enriched, enriched.ResourceLimits)
	if err != nil {
		t.Fatal(err)
	}

	solvedAt := -1
	for i, candidate := range candidates {
		proposal, buildErr := builder.Build(candidate, enriched)
		if buildErr != nil {
			continue
		}
		program := pArtifactProgram(proposal.Artifact)
		if programFits(program, enrichedHidden) {
			solvedAt = i + 1
			break
		}
	}
	if solvedAt < 1 {
		t.Fatal("promoted representation did not change executable generator space enough to solve the hidden task")
	}

	// Baseline control: the original scalar generator remains unable to solve
	// the held-out target, so the observed finite acquisition effort is caused
	// by the promoted representation rather than a silent baseline improvement.
	baseCandidates, err := (UniversalMechanismSearch{}).SearchMechanisms(spec, spec.ResourceLimits)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range baseCandidates {
		proposal, buildErr := builder.Build(candidate, spec)
		if buildErr != nil {
			continue
		}
		if programFits(pArtifactProgram(proposal.Artifact), hidden) {
			t.Fatalf("baseline scalar generator solved target through %q after promotion", candidate.Mechanism)
		}
	}
}
