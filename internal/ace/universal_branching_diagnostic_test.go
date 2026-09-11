package ace

import "testing"

func TestUniversalBranchingSynthesizesThresholdDiagnostic(t *testing.T) {
	cases := []ProgramTestCase{methodInputOutputExample(-8,0),methodInputOutputExample(0,0),methodInputOutputExample(1,0),methodInputOutputExample(5,1)}
	spec, err := GeneralCapabilitySpecification(Task{ID:"diagnostic-threshold",Goal:"conditional transform",Requirements:[]string{"x"},Structure:[]string{"scalar","conditional"},Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20}},cases)
	if err != nil { t.Fatal(err) }
	cs, err := (UniversalMechanismSearch{}).SearchMechanisms(spec,spec.ResourceLimits)
	if err != nil { t.Fatal(err) }
	for _, c := range cs {
		if c.Mechanism != "universal:branching" { continue }
		p, err := (UniversalProgramBuilder{}).Build(c,spec)
		if err != nil { t.Fatalf("branching synthesis failed: %v",err) }
		if !programFitsJSON(p.Artifact,cases) { t.Fatal("serialized branching program failed behavioral validation") }
		return
	}
	t.Fatal("branching mechanism missing")
}
