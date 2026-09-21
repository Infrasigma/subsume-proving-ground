package ace

import "testing"

func TestUniversalContractsAndSynthesis(t *testing.T) {
	task := Task{ID:"add-one",Goal:"y=x+1",Budget:ResourceVector{Search:500,Verify:500}}
	cases := []ProgramTestCase{
		{Input:map[string]string{"x":"0"},Expected:map[string]string{"y":"1"}},
		{Input:map[string]string{"x":"2"},Expected:map[string]string{"y":"3"}},
	}
	spec, err := GeneralCapabilitySpecification(task,cases)
	if err != nil { t.Fatal(err) }
	candidates, err := (UniversalMechanismSearch{}).SearchMechanisms(spec,task.Budget)
	if err != nil || len(candidates)==0 { t.Fatalf("search failed: %v",err) }
	for _, candidate := range candidates {
		if proposal, err := (UniversalProgramBuilder{}).Build(candidate,spec); err == nil {
			if proposal.Artifact == "" { t.Fatal("empty synthesis artifact") }
			return
		}
	}
	t.Fatal("all universal synthesis strategies failed")
}
