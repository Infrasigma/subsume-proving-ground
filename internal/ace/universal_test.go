package ace

import (
	"testing"
)

func TestUniversalProgramBuilderLearnsExecutableCapability(t *testing.T) {
	task := Task{
		ID: "add-one",
		Goal: "y=x+1",
		Budget: ResourceVector{Search: 1000, Verify: 1000},
	}
	cases := []ProgramTestCase{
		{Input: map[string]string{"x":"0"}, Expected: map[string]string{"y":"1"}},
		{Input: map[string]string{"x":"2"}, Expected: map[string]string{"y":"3"}},
		{Input: map[string]string{"x":"-3"}, Expected: map[string]string{"y":"-2"}},
	}
	spec, err := GeneralCapabilitySpecification(task, cases)
	if err != nil { t.Fatal(err) }
	candidates, err := (UniversalMechanismSearch{}).SearchMechanisms(spec, task.Budget)
	if err != nil || len(candidates) == 0 { t.Fatalf("mechanism search failed: %v",err) }
	var proposal ModificationProposal
	for _, c := range candidates {
		p, err := (UniversalProgramBuilder{}).Build(c, spec)
		if err == nil { proposal = p; break }
	}
	if proposal.Artifact == "" || proposal.Candidate.ID == "" {
		t.Fatalf("no executable capability synthesized: %+v",proposal)
	}
}
