package acex

import "testing"

func TestV9CEGISRejectsOverfittingCandidate(t *testing.T) {
	x := v4IntInput()
	truth := v4IntExpr("abs", x)
	task := V4Task{
		ID:"v9-overfit",
		InputType:V4Int, OutputType:V4Int,
		TrainInputs:[]V4Value{{Type:V4Int,Int:1},{Type:V4Int,Int:2}},
		TrainOutput:[]V4Value{{Type:V4Int,Int:1},{Type:V4Int,Int:2}},
	}
	res, err := V9CEGIS(truth,task,NewV4Library(),7,600,-8,8,6)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Certificate.FinalVerified {
		t.Fatalf("candidate lacks final verification: %+v",res.Certificate)
	}
	if len(res.FailureMemory)==0 {
		t.Fatal("CEGIS never recorded a counterexample; likely accepted an overfit candidate")
	}
	if res.Certificate.Rounds < 2 {
		t.Fatalf("CEGIS did not refine after a counterexample: %+v",res.Certificate)
	}
	counter, found, err := V9OracleCounterexample(res.Program,truth,NewV4Library(),-8,8)
	if err != nil || found {
		t.Fatalf("final candidate still has bounded counterexample: found=%v c=%+v err=%v",found,counter,err)
	}
	if res.Certificate.Digest == "" {
		t.Fatal("missing verification certificate digest")
	}
}

func TestV9MechanismImprovementUsesExecutableCEGIS(t *testing.T) {
	x := v4IntInput()
	truth := v4IntExpr("abs",x)
	task := V4Task{
		ID:"v9-mechanism",
		InputType:V4Int,OutputType:V4Int,
		TrainInputs:[]V4Value{{Type:V4Int,Int:1},{Type:V4Int,Int:3}},
		TrainOutput:[]V4Value{{Type:V4Int,Int:1},{Type:V4Int,Int:3}},
	}
	mech,res,err := V9CounterexampleDrivenImprovement(truth,task,NewV4Library())
	if err != nil { t.Fatal(err) }
	if !mech.UseCEGIS || !res.Certificate.FinalVerified {
		t.Fatalf("mechanism was not actual CEGIS: mech=%+v cert=%+v",mech,res.Certificate)
	}
}
