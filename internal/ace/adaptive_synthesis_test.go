package ace

import (
	"context"
	"testing"
)

func TestAdaptiveUniversalSynthesisUsesCandidateAndDeterministicBudget(t *testing.T) {
	train := []ProgramTestCase{
		{Input: map[string]string{"x": "-6"}, Expected: map[string]string{"y": "-5"}},
		{Input: map[string]string{"x": "-1"}, Expected: map[string]string{"y": "0"}},
		{Input: map[string]string{"x": "0"}, Expected: map[string]string{"y": "0"}},
		{Input: map[string]string{"x": "3"}, Expected: map[string]string{"y": "6"}},
		{Input: map[string]string{"x": "8"}, Expected: map[string]string{"y": "16"}},
	}
	hidden := []ProgramTestCase{
		{Input: map[string]string{"x": "-9"}, Expected: map[string]string{"y": "-8"}},
		{Input: map[string]string{"x": "4"}, Expected: map[string]string{"y": "8"}},
	}
	spec, err := GeneralCapabilitySpecification(
		Task{ID: "adaptive-synthesis-handoff", Goal: "piecewise transform"},
		train,
	)
	if err != nil {
		t.Fatal(err)
	}
	candidate := ArchitectureCandidate{
		ID:        Hash([]any{"adaptive-synthesis-handoff", "branching"}),
		Mechanism: "universal:branching",
		Resources: spec.ResourceLimits,
	}
	proposal, err := AdaptiveUniversalSynthesisWithContext(
		context.Background(),
		candidate,
		spec,
		AdaptiveUniversalSynthesisMaxExpansions,
	)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Candidate.Mechanism != candidate.Mechanism {
		t.Fatalf("synthesis handoff dropped candidate mechanism: got %q want %q", proposal.Candidate.Mechanism, candidate.Mechanism)
	}
	var program UniversalProgram
	if err := unmarshalJSON([]byte(proposal.Artifact), &program); err != nil {
		t.Fatal(err)
	}
	if !programFits(program, hidden) {
		t.Fatal("handoff synthesis failed hidden verification")
	}
}
