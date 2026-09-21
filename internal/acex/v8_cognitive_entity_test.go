package acex

import (
	"testing"

	"github.com/Infrasigma/subsume-proving-ground/internal/ace"
)

func TestV8UnifiedCognitiveEntity(t *testing.T) {
	entity := NewV8CognitiveEntity()

	visible, err := makeV6TaskFamily(11, []int{-3,0,3}, true)
	if err != nil { t.Fatal(err) }
	future, err := makeV6TaskFamily(12, []int{-4,-1,2,4}, false)
	if err != nil { t.Fatal(err) }
	result, err := entity.LearnStatic(visible, future, 9, 1200)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Added) != 1 || !entity.HasEvidence("semantic-prospective-abstraction") {
		t.Fatalf("static abstraction was not integrated: %+v caps=%v", result.Added, entity.VerifiedCapabilities())
	}

	hidden, err := makeV6TaskFamily(13, []int{-5,-2,1,5}, true)
	if err != nil { t.Fatal(err) }
	if err := entity.SelectMechanism(visible, hidden, 9, 1200); err != nil {
		t.Fatal(err)
	}
	if entity.ActiveStrategy.Strategy != V6SemanticSearch || len(entity.StrategyHistory) != 1 {
		t.Fatalf("selected mechanism was not persisted: %+v history=%d", entity.ActiveStrategy, len(entity.StrategyHistory))
	}
	if _, err := entity.SolveStatic(future[0], 9, 1200); err != nil {
		t.Fatal(err)
	}
	if err := entity.RollbackMechanism(); err != nil {
		t.Fatal(err)
	}
	if entity.ActiveStrategy.Strategy != V6BaselineSearch {
		t.Fatalf("strategy rollback did not restore baseline: %+v", entity.ActiveStrategy)
	}

	h := []V5Hypothesis{
		{ID:"h0", Prior:.5, Outcome:map[string]string{"probe-a":"bad","probe-b":"same"}},
		{ID:"h1", Prior:.5, Outcome:map[string]string{"probe-a":"good","probe-b":"same"}},
	}
	if err := entity.Inquire(h,"",""); err != nil {
		t.Fatal(err)
	}
	if !entity.HasEvidence("causal-inquiry") {
		t.Fatal("causal inquiry evidence missing")
	}
	proposal, err := entity.InventTool(
		ace.Task{ID:"tool-add-one",Goal:"y=x+1",Budget:ace.ResourceVector{Search:1000,Verify:1000}},
		[]ace.ProgramTestCase{
			{Input:map[string]string{"x":"0"},Expected:map[string]string{"y":"1"}},
			{Input:map[string]string{"x":"4"},Expected:map[string]string{"y":"5"}},
			{Input:map[string]string{"x":"-2"},Expected:map[string]string{"y":"-1"}},
		},
		[]ace.ProgramTestCase{
			{Input:map[string]string{"x":"7"},Expected:map[string]string{"y":"8"}},
			{Input:map[string]string{"x":"-5"},Expected:map[string]string{"y":"-4"}},
		},
	)
	if err != nil || proposal.Artifact == "" || !entity.HasEvidence("tool-invention") {
		t.Fatalf("symbolic tool invention failed: err=%v proposal=%+v caps=%v",err,proposal,entity.VerifiedCapabilities())
	}
	if err := entity.Inquire(h,"probe-a","good"); err != nil {
		t.Fatal(err)
	}

	s0,s1,s2,s3 := v7State("entity",0),v7State("entity",1),v7State("entity",2),v7State("entity",3)
	if _, err := entity.ObserveAndAct(s0,[]string{"A","B","C"}); err != nil {
		t.Fatal(err)
	}
	entity.ObserveOutcome(s0,"A",s1,0,false)
	if _, err := entity.ObserveAndAct(s1,[]string{"A","B","C"}); err != nil {
		t.Fatal(err)
	}
	entity.ObserveOutcome(s1,"B",s2,0,false)
	if _, err := entity.ObserveAndAct(s2,[]string{"A","B","C"}); err != nil {
		t.Fatal(err)
	}
	entity.ObserveOutcome(s2,"C",s3,1,true)

	if !entity.HasEvidence("interactive-action-selection") ||
		!entity.HasEvidence("verified-interactive-experience") {
		t.Fatalf("interactive cognition not integrated: %v", entity.VerifiedCapabilities())
	}

	traces := [][]V7Step{{
		{Before:V7StateKey(s0),Action:"A",After:V7StateKey(s1)},
		{Before:V7StateKey(s1),Action:"B",After:V7StateKey(s2)},
	}, {
		{Before:V7StateKey(v7State("other",0)),Action:"X",After:V7StateKey(v7State("other",1))},
		{Before:V7StateKey(v7State("other",1)),Action:"Y",After:V7StateKey(v7State("other",2))},
	}}
	if err := entity.ConsolidateInteractive(traces); err != nil {
		t.Fatal(err)
	}
	if !entity.HasEvidence("procedural-consolidation") {
		t.Fatal("procedural consolidation not integrated")
	}
}

func TestV8SurpriseMemoryAffectsRetrieval(t *testing.T) {
	entity := NewV8CognitiveEntity()
	_ = entity.Remember(V5MemoryTrace{
		ID:"routine", Context:[]string{"same","context"},
		PredictionErr:.05, Utility:.2, Verified:true,
	})
	_ = entity.Remember(V5MemoryTrace{
		ID:"surprise", Context:[]string{"same","context"},
		PredictionErr:.95, Utility:.2, Failure:true, Verified:true,
	})
	got := entity.Retrieve([]string{"same","context"},1)
	if len(got)!=1 || got[0].ID!="surprise" {
		t.Fatalf("surprise memory did not dominate retrieval: %+v",got)
	}
	if !entity.HasEvidence("persistent-surprise-memory") {
		t.Fatal("memory evidence missing")
	}
}

func TestV8FailureMemoryBlocksRepeatedBadAction(t *testing.T) {
	entity := NewV8CognitiveEntity()
	s0 := v7State("memory",0)
	_ = entity.Remember(V5MemoryTrace{
		ID:"bad-action", Context:[]string{V7StateKey(s0),"BAD"},
		PredictionErr:.95, Utility:-1, Failure:true, Verified:true,
	})
	action, err := entity.ObserveAndAct(s0,[]string{"BAD","GOOD"})
	if err != nil { t.Fatal(err) }
	if action == "BAD" {
		t.Fatalf("failure memory did not alter action selection: %q", action)
	}
}
