package acex

import "testing"

func graphA() RelationalState {
	return RelationalState{
		Nodes: []RelNode{
			{ID:"a",Kind:"agent",Attrs:map[string]string{"role":"source"}},
			{ID:"b",Kind:"agent",Attrs:map[string]string{"role":"middle"}},
			{ID:"c",Kind:"agent",Attrs:map[string]string{"role":"sink"}},
		},
		Edges: []RelEdge{
			{From:"a",To:"b",Kind:"causes"},
			{From:"b",To:"c",Kind:"causes"},
		},
	}
}

func graphRenamed() RelationalState {
	return RelationalState{
		Nodes: []RelNode{
			{ID:"node-7",Kind:"agent",Attrs:map[string]string{"role":"source"}},
			{ID:"node-9",Kind:"agent",Attrs:map[string]string{"role":"middle"}},
			{ID:"node-2",Kind:"agent",Attrs:map[string]string{"role":"sink"}},
		},
		Edges: []RelEdge{
			{From:"node-9",To:"node-2",Kind:"causes"},
			{From:"node-7",To:"node-9",Kind:"causes"},
		},
	}
}

func TestV2RelationalInvariantTransfer(t *testing.T) {
	a := graphA()
	b := graphRenamed()
	ha := WLInvariant(a, 3)
	hb := WLInvariant(b, 3)
	if ha != hb {
		t.Fatalf("renaming changed relational invariant: %s != %s", ha, hb)
	}
	m := RelationalMemory{}
	m.Add(RelationalConcept{ID:"causal-chain",Fingerprint:ha,Support:10,Confidence:1})
	got, ok := m.Match(b, 3)
	if !ok || got.ID != "causal-chain" {
		t.Fatalf("relational memory failed renamed-symbol transfer: %+v %v", got, ok)
	}

	// Nuisance node must not be able to alter the learned core fingerprint
	// only when it is disconnected; this tests selective relational scope.
	distractor := b
	distractor.Nodes = append(distractor.Nodes, RelNode{ID:"noise",Kind:"junk"})
	hb2 := WLInvariant(distractor, 3)
	if hb2 == hb {
		t.Fatal("global invariant unexpectedly ignored a disconnected object")
	}
}
