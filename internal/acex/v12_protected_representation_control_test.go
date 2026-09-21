package acex

import "testing"

func TestV12ProtectedRepresentationFamilyOracleControl(t *testing.T) {
	r := NewV11DirectedExecutableRepresentation()
	for i := 0; i < 6; i++ {
		state, actions, correct := v12ProtectedState(12000+i*97, i%2, 3+(i%2))
		for _, action := range actions {
			if action == correct {
				r.Record(state, action, 1, true)
			} else {
				r.Record(state, action, -1, false)
			}
		}
	}
	if !r.Synthesize() {
		t.Fatalf("oracle-labeled examples are not separable: valid=%t nodes=%d edges=%d examples=%d expansions=%d",
			r.Valid, r.Nodes, len(r.Edges), len(r.Examples), r.SearchExpansions)
	}
}
