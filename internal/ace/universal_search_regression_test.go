package ace

import "testing"

func TestUniversalBranchingBuilderBoundedFrontier(t *testing.T) {
	task := Task{ID: "piecewise-clamp-regression", Goal: "clamp negative values to zero"}
	examples := []ProgramTestCase{
		{Input: map[string]string{"x": "-3"}, Expected: map[string]string{"y": "0"}},
		{Input: map[string]string{"x": "4"}, Expected: map[string]string{"y": "4"}},
	}
	spec, err := GeneralCapabilitySpecification(task, examples)
	if err != nil { t.Fatal(err) }
	cs, err := UniversalMechanismSearch{}.SearchMechanisms(spec, ResourceVector{TimeMS: 10000})
	if err != nil { t.Fatal(err) }
	var branch ArchitectureCandidate
	for _, c := range cs {
		if c.Mechanism == "universal:branching" { branch = c; break }
	}
	if branch.Mechanism == "" { t.Fatal("branching strategy missing") }
	proposal, err := UniversalProgramBuilder{}.Build(branch, spec)
	if err != nil { t.Fatal(err) }
	if proposal.Artifact == "" { t.Fatal("branching builder produced no executable artifact") }
}
