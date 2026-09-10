package ace

import (
	"errors"
	"fmt"
)

func RunRecursiveCapabilityProtocolV3() (map[string]float64, error) {
	_, c1, _, err := (AffineFamily{}).Generate(0, false); if err != nil { return nil, err }
	k1, err := ParameterizedMechanismSearch(c1); if err != nil { return nil, err }
	_, c2, _, err := (ThresholdFamily{}).Generate(0, false); if err != nil { return nil, err }
	if _, err = ParameterizedMechanismSearch(c2); err == nil { return nil, errors.New("pre-improvement arithmetic method unexpectedly solved conditional task") }
	spec2, err := GeneralCapabilitySpecification(Task{ID:"opaque-t2",Goal:"classify input",Requirements:[]string{"x"},Structure:[]string{"scalar","conditional"},Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20}},c2);if err!=nil{return nil,err}
	hidden2:=[]ProgramTestCase{methodInputOutputExample(-8,0),methodInputOutputExample(0,0),methodInputOutputExample(1,0),methodInputOutputExample(5,1)}
	telemetry:=AcquisitionTelemetry{TaskID:"opaque-t2",TaskStructure:[]string{"scalar","conditional"},KnownExamples:len(c2),CandidateCount:1,CandidateFailures:[]string{"arithmetic candidate rejected by independent boundary counterexample"},Counterexamples:1,Representation:[]string{"scalar-input-output"},SearchPath:[]string{"parameterized-add","parameterized-mul"},VerificationOutcomes:[]string{"independent-counterexample-failed"},Cost:ResourceVector{Compute:1,ExperimentBudget:1}}
	method,diagnosis,evals,err:=AutonomousMethodImprovement(telemetry,spec2,hidden2,nil,nil);if err!=nil{return nil,fmt.Errorf("autonomous method improvement failed: %w",err)}
	registry:=InstalledMethodRegistry{};if err:=registry.Install(method);err!=nil{return nil,err};futureCandidates,err:=registry.Apply(spec2);if err!=nil{return nil,err};if len(futureCandidates)==0||futureCandidates[0].Mechanism!="universal:branching"{return nil,errors.New("installed method did not change future acquisition behavior")}
	c3:=[]ProgramTestCase{methodInputOutputExample(-7,-6),methodInputOutputExample(-1,0),methodInputOutputExample(0,0),methodInputOutputExample(4,8),methodInputOutputExample(9,18)}
	spec3,err:=GeneralCapabilitySpecification(Task{ID:"opaque-t3",Goal:"piecewise transform",Requirements:[]string{"x"},Structure:[]string{"scalar","piecewise","branch-plus-arithmetic"},Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20}},c3);if err!=nil{return nil,err}
	if _,err:=ParameterizedMechanismSearch(c3);err==nil{return nil,errors.New("K0 arithmetic search solved future piecewise task")};k2Candidates,err:=registry.Apply(spec3);if err!=nil{return nil,err};if len(k2Candidates)==0||k2Candidates[0].Mechanism!="universal:branching"{return nil,errors.New("installed M1 was not used on future task")};var solved bool;for _,candidate:=range k2Candidates{p,e:=(UniversalProgramBuilder{}).Build(candidate,spec3);if e==nil&&programFitsJSON(p.Artifact,c3){solved=true;break}};if !solved{return nil,errors.New("K2 failed future piecewise acquisition")}
	return map[string]float64{"verified":1,"K0_future_solved":0,"K2_future_solved":1,"method_candidates":float64(len(evals)),"method_diagnosis_confidence":diagnosis.Confidence,"method_installed":1,"future_search_changed":1,"conditional_signal":1,"prior_k1":func()float64{if k1.Verified{return 1};return 0}()},nil
}

// RunReplicatedCompoundingV2 is retained as a scientific guard. The former
// replicated family is algebraically affine (y=2x+5), so a universal baseline
// can solve it without the retained composition. Returning invalidation rather
// than a fabricated win keeps the evidence ledger honest.
func RunReplicatedCompoundingV2(n int) (map[string]float64, error) {
	if n < 2 { return nil, errors.New("need replication") }
	return map[string]float64{"repetitions":float64(n),"valid":0,"baseline_degenerate":1,"wins":0,"all_verified":0},nil
}
