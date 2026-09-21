package acex

import "testing"

func v7State(prefix string, stage int) RelationalState {
	return RelationalState{
		Nodes: []RelNode{{ID:prefix+"-world", Kind:"world", Attrs:map[string]string{"stage":string(rune('0'+stage))}}},
	}
}

func TestV7ExplorationGoalPlanningAndPersistence(t *testing.T) {
	agent := NewV7CognitiveAgent()
	s0 := v7State("a",0)
	s1 := v7State("a",1)
	s2 := v7State("a",2)
	s3 := v7State("a",3)
	available := []string{"A","B","C"}

	a1, err := agent.NextAction(s0, available)
	if err != nil || a1 != "A" { t.Fatalf("unexpected first exploration action: %q %v", a1, err) }
	agent.ExecuteObserved(s0,a1,s1,0,false)

	a2, err := agent.NextAction(s1, available)
	if err != nil || a2 != "B" { t.Fatalf("unexpected second exploration action: %q %v", a2, err) }
	agent.ExecuteObserved(s1,a2,s2,0,false)

	a3, err := agent.NextAction(s2, available)
	if err != nil || a3 != "C" { t.Fatalf("unexpected third exploration action: %q %v", a3, err) }
	agent.ExecuteObserved(s2,a3,s3,1,true)

	goal, ok := agent.InferGoal()
	if !ok || !goal.Terminal || goal.StateKey != V7StateKey(s3) {
		t.Fatalf("goal was not acquired from feedback: %+v %v", goal, ok)
	}

	plan, err := agent.Graph.Plan(V7StateKey(s0),func(e V7Step)bool{return e.After==V7StateKey(s3)})
	if err != nil { t.Fatal(err) }
	if !plan.Verified || len(plan.Actions)!=3 {
		t.Fatalf("verified experience plan invalid: %+v", plan)
	}

	if err := agent.ObserveUnexpected(s1,"B",V7StateKey(s2),s3); err != nil {
		t.Fatal(err)
	}
	if len(agent.Failures)==0 {
		t.Fatal("unexpected transition was not detected")
	}

	// Delete the imagined raw episode. The verified experience graph remains.
	raw := [][]V7Step{{
		{Before:V7StateKey(s0),Action:"A",After:V7StateKey(s1)},
		{Before:V7StateKey(s1),Action:"B",After:V7StateKey(s2)},
		{Before:V7StateKey(s2),Action:"C",After:V7StateKey(s3),Reward:1,Terminal:true},
	}}
	raw = nil
	_ = raw
	plan2, err := agent.Graph.Plan(V7StateKey(s0),func(e V7Step)bool{return e.After==V7StateKey(s3)})
	if err != nil || len(plan2.Actions)!=3 {
		t.Fatalf("persistent planning failed after raw episode deletion: %+v %v", plan2, err)
	}
}

func TestV7MacroInductionAndStructuralActionTransfer(t *testing.T) {
	s0a,s1a,s2a := v7State("left",0),v7State("left",1),v7State("left",2)
	s0b,s1b,s2b := v7State("right",0),v7State("right",1),v7State("right",2)
	traces := [][]V7Step{
		{
			{Before:V7StateKey(s0a),Action:"OPEN",After:V7StateKey(s1a)},
			{Before:V7StateKey(s1a),Action:"PUSH",After:V7StateKey(s2a)},
		},
		{
			{Before:V7StateKey(s0b),Action:"ACT1",After:V7StateKey(s1b)},
			{Before:V7StateKey(s1b),Action:"ACT2",After:V7StateKey(s2b)},
		},
	}
	agent := NewV7CognitiveAgent()
	agent.Consolidate(traces)
	if len(agent.Macros)==0 {
		t.Fatal("no repeated verified macro discovered")
	}
	if len(agent.Macros[0].Actions)!=2 {
		t.Fatalf("unexpected macro: %+v",agent.Macros[0])
	}

	step := traces[0][0]
	got, ok := V7FindTransferredAction(step,s0b,traces[1])
	if !ok || got!="ACT1" {
		t.Fatalf("structural action-role transfer failed: %q %v",got,ok)
	}

	compressed := V7ApplyMacro([]string{"OPEN","PUSH","OPEN","PUSH"},agent.Macros[0])
	if len(compressed) != 2 {
		t.Fatalf("macro compression failed: %v",compressed)
	}
}

func TestV7ExplorationDoesNotPretendUnobservedTransitionsAreKnown(t *testing.T) {
	graph := NewV7ExperienceGraph()
	s0 := v7State("x",0)
	s1 := v7State("x",1)
	key := graph.ObserveState(s0)
	graph.AddStep(s0,"A",s1,0,false)
	if _, ok := V7KnownTransition(graph,key,"B"); ok {
		t.Fatal("unobserved transition was treated as known")
	}
	if _, err := graph.Plan(key,func(e V7Step)bool{return e.Action=="B"}); err == nil {
		t.Fatal("planner invented an unobserved action effect")
	}
}
