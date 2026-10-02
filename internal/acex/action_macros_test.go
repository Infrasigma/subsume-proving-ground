package acex

import "testing"

func TestV2HierarchicalProceduralMemory(t *testing.T) {
	m:=NewPredictiveModel()
	for x:=0;x<6;x++ {
		if err:=m.Observe(Transition{
			Before:NumericState{"x":x},
			Action:"inc",
			After:NumericState{"x":x+1},
		}); err!=nil { t.Fatal(err) }
	}

	traces:=[][]string{
		{"inc","inc","inc","emit"},
		{"scan","inc","inc","inc","store"},
		{"observe","inc","inc","inc","write"},
	}
	macros:=commonActionMacros(traces,2)
	if len(macros)==0 { t.Fatal("no action macro discovered") }
	var macro ActionMacro
	for _,x:=range macros {
		if len(x.Actions)>=3 { macro=x; break }
	}
	if len(macro.Actions)!=3 { t.Fatalf("expected 3-action macro, got %+v",macro) }

	expanded,conf,err:=PredictMacro(m,NumericState{"x":0},macro)
	if err!=nil { t.Fatal(err) }
	if expanded["x"]!=3 || conf<0.5 { t.Fatalf("bad macro prediction: state=%v conf=%v",expanded,conf) }

	plain,err:=PlanWithActionLibrary(
		m,NumericState{"x":0},nil,[]string{"inc"},
		func(s NumericState) bool { return s["x"]>=3 },5,
	)
	if err!=nil { t.Fatal(err) }

	hier,err:=PlanWithActionLibrary(
		m,NumericState{"x":0},[]ActionMacro{macro},[]string{"inc"},
		func(s NumericState) bool { return s["x"]>=3 },2,
	)
	if err!=nil { t.Fatal(err) }
	if hier.Cost>=plain.Cost {
		t.Fatalf("procedural macro did not reduce planning depth: plain=%+v hier=%+v",plain,hier)
	}
	if len(hier.Actions)!=1 || hier.Actions[0]!=macro.ID {
		t.Fatalf("planner did not invoke procedural macro: %+v",hier)
	}

	// A stale/unverified macro must not be admitted to planning.
	bad:=ActionMacro{ID:"bad",Actions:[]string{"unknown","inc"},Uses:9}
	if _,_,err:=PredictMacro(m,NumericState{"x":0},bad); err==nil {
		t.Fatal("planner accepted an unverified macro")
	}
}
