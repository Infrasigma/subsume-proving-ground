package acex

import (
	"encoding/json"
	"testing"
)

func TestV5GroundingMemoryAndCausalLoop(t *testing.T) {
	rawA := []byte(`{
		"entities":[
			{"id":"a","kind":"agent","attrs":{"role":"source"}},
			{"id":"b","kind":"agent","attrs":{"role":"middle"}},
			{"id":"c","kind":"agent","attrs":{"role":"sink"}}
		],
		"relations":[
			{"from":"a","to":"b","kind":"causes"},
			{"from":"b","to":"c","kind":"causes"}
		]
	}`)
	rawB := []byte(`{
		"entities":[
			{"id":"u9","kind":"agent","attrs":{"role":"middle"}},
			{"id":"u1","kind":"agent","attrs":{"role":"source"}},
			{"id":"u7","kind":"agent","attrs":{"role":"sink"}}
		],
		"relations":[
			{"from":"u1","to":"u9","kind":"causes"},
			{"from":"u9","to":"u7","kind":"causes"}
		]
	}`)
	g := GroundingRuntime{Grounder: JSONGrounder{}}
	a, err := g.Observe(rawA)
	if err != nil {
		t.Fatal(err)
	}
	b, err := g.Observe(rawB)
	if err != nil {
		t.Fatal(err)
	}
	if a.Digest != b.Digest {
		t.Fatalf("grounding is not identifier invariant: %s != %s", a.Digest, b.Digest)
	}

	mem := V5AdaptiveMemory{}
	if err := mem.Record(V5MemoryTrace{ID:"low", Context:[]string{"a","b"}, PredictionErr:.05, Utility:.2, Verified:true}); err != nil {
		t.Fatal(err)
	}
	if err := mem.Record(V5MemoryTrace{ID:"surprising-failure", Context:[]string{"a","b"}, PredictionErr:.95, Utility:.2, Failure:true, Verified:true}); err != nil {
		t.Fatal(err)
	}
	got := mem.Retrieve([]string{"a","b"}, 1)
	if len(got) != 1 || got[0].ID != "surprising-failure" {
		t.Fatalf("surprise/failure did not alter retrieval priority: %+v", got)
	}

	h := []V5Hypothesis{
		{ID:"h0", Prior:.33, Outcome:map[string]string{"x":"red","y":"blue"}},
		{ID:"h1", Prior:.33, Outcome:map[string]string{"x":"green","y":"blue"}},
		{ID:"h2", Prior:.34, Outcome:map[string]string{"x":"yellow","y":"blue"}},
	}
	plan, err := V5ChooseIntervention(h)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != "x" || plan.Disagreement != 3 {
		t.Fatalf("failed to select maximally discriminating intervention: %+v", plan)
	}
	post, err := V5ReviseHypotheses(h, plan.Action, "green")
	if err != nil {
		t.Fatal(err)
	}
	if len(post) != 1 || post[0].ID != "h1" {
		t.Fatalf("belief revision did not falsify incompatible hypotheses: %+v", post)
	}

	var check map[string]any
	if err := json.Unmarshal([]byte("{\"digest\":\""+a.Digest+"\"}"), &check); err != nil {
		t.Fatal(err)
	}
}

func TestV5MechanismTerminal(t *testing.T) {
	seeds := []int64{610117, 830921, 941173}
	runtime := NewV5MechanismRuntime()
	for block, seed := range seeds {
		visible := V5IndependentSuite(seed, block)
		hidden := V5IndependentSuite(seed+1000003, block+11)

		base := V5EvaluateMechanism(runtime.Active, visible)
		if !base.Verified {
			t.Fatalf("block=%d baseline failed visible suite: %+v", block, base)
		}

		selected, err := V5SelectMechanism(runtime.Active, visible, "representation")
		if err != nil {
			t.Fatalf("block=%d mechanism meta-search failed: %v", block, err)
		}
		if selected.Candidate.Key() == runtime.Active.Key() {
			t.Fatalf("block=%d selector returned unchanged mechanism", block)
		}

		if err := runtime.TryPromote(selected.Candidate, visible, hidden); err != nil {
			t.Fatalf("block=%d proof-carrying promotion failed: %v", block, err)
		}
		receipt := runtime.Promotions[len(runtime.Promotions)-1]
		if !receipt.Verified || receipt.NewVersion != receipt.ParentVersion+1 {
			t.Fatalf("block=%d invalid promotion receipt: %+v", block, receipt)
		}
		for i, ratio := range receipt.HiddenRatios {
			if ratio >= .80 {
				t.Fatalf("block=%d hidden ratio[%d]=%.3f", block, i, ratio)
			}
		}
		if err := runtime.Rollback(); err != nil {
			t.Fatalf("block=%d rollback failed: %v", block, err)
		}
		if runtime.Version != 0 {
			t.Fatalf("block=%d runtime version not restored: %d", block, runtime.Version)
		}

		challenge := V5NextChallenge([]string{"transfer"}, seed+17)
		if challenge.Gap != "transfer" || challenge.ID == "" || challenge.Difficulty <= 1 {
			t.Fatalf("block=%d bad endogenous challenge: %+v", block, challenge)
		}
	}
}

func TestV5PersistenceAfterRawEpisodeDeletion(t *testing.T) {
	task := V5IndependentSuite(610117, 0)[0]
	lab := RepresentationLab{MaxAtoms:4, Policy:PolicySpecific}
	concept, _, err := lab.Discover(task.Train, task.Holdout)
	if err != nil {
		t.Fatal(err)
	}
	store := KnowledgeStore{}
	if err := store.Add(VerifiedKnowledge{Concept:concept, Verified:true}); err != nil {
		t.Fatal(err)
	}
	task.Train.Examples = nil
	task.Holdout.Examples = nil
	retained := store.Items[0]
	if !retained.Verified || len(retained.Concept.Features) < 3 {
		t.Fatalf("verified abstraction was not retained after episode deletion: %+v", retained)
	}
}
