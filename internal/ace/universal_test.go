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


func TestUniversalBuilderGeneralizesAddOne(t *testing.T) {
	task := Task{ID:"add-one-hidden", Goal:"y=x+1", Budget:ResourceVector{Search:1000, Verify:1000}}
	train := []ProgramTestCase{
		{Input:map[string]string{"x":"0"},Expected:map[string]string{"y":"1"}},
		{Input:map[string]string{"x":"4"},Expected:map[string]string{"y":"5"}},
		{Input:map[string]string{"x":"-2"},Expected:map[string]string{"y":"-1"}},
	}
	holdout := []ProgramTestCase{
		{Input:map[string]string{"x":"7"},Expected:map[string]string{"y":"8"}},
		{Input:map[string]string{"x":"-5"},Expected:map[string]string{"y":"-4"}},
	}
	spec, err := GeneralCapabilitySpecification(task, train)
	if err != nil { t.Fatal(err) }
	candidates, err := (UniversalMechanismSearch{}).SearchMechanisms(spec, task.Budget)
	if err != nil { t.Fatal(err) }
	var found bool
	for _, candidate := range candidates {
		proposal, err := (UniversalProgramBuilder{}).Build(candidate, spec)
		if err != nil {
			continue
		}
		var program UniversalProgram
		if err := json.Unmarshal([]byte(proposal.Artifact), &program); err != nil {
			t.Fatal(err)
		}
		t.Logf("candidate=%s artifact=%s holdout=%v", candidate.Mechanism, proposal.Artifact, ProgramFitsForTests(program, holdout))
		if ProgramFitsForTests(program, holdout) {
			found = true
			break
		}
	}
	if !found { t.Fatal("no universal synthesis candidate generalized from train to holdout") }
}
