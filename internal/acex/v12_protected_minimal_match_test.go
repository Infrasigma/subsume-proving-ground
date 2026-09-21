package acex

import "testing"

func TestV12ProtectedMinimalDirectedPatternMatcher(t *testing.T) {
	p := V11DirectedExecutableRepresentation{
		Root:  0,
		Nodes: 2,
		Edges: []V11DirectedPatternEdge{{From: 0, To: 1}},
		Valid: true,
	}
	state, actions, correct := v12ProtectedState(12000, 0, 3)
	if !p.Match(state, correct) {
		t.Fatalf("minimal root->child pattern did not match correct action %q", correct)
	}
	for _, action := range actions {
		if action == correct {
			continue
		}
		if p.Match(state, action) {
			t.Fatalf("minimal root->child pattern incorrectly matched negative action %q", action)
		}
	}
}
