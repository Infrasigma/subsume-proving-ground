package acex

import "testing"

func v12ProtectedState(seed, generator, width int) (RelationalState, []string, string) {
	actions := make([]string, width)
	for i := range actions {
		actions[i] = "g" + itoa(generator) + "-" + itoa(seed) + "-" + string(rune('a'+i))
	}
	correct := actions[(seed/7+generator)%width]
	nodes := make([]RelNode, 0, width*4)
	edges := make([]RelEdge, 0, width*3)
	for _, action := range actions {
		x, y, z := action+"-x", action+"-y", action+"-z"
		nodes = append(nodes,
			RelNode{ID: action, Kind: "action"},
			RelNode{ID: x, Kind: "node"},
			RelNode{ID: y, Kind: "node"},
			RelNode{ID: z, Kind: "node"},
		)
		if action == correct {
			edges = append(edges,
				RelEdge{From: action, To: x, Kind: "r"},
				RelEdge{From: x, To: y, Kind: "r"},
				RelEdge{From: y, To: z, Kind: "r"},
			)
		} else {
			edges = append(edges,
				RelEdge{From: z, To: y, Kind: "r"},
				RelEdge{From: y, To: x, Kind: "r"},
				RelEdge{From: x, To: action, Kind: "r"},
			)
		}
	}
	return RelationalState{Nodes: nodes, Edges: edges}, actions, correct
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func TestV12ProtectedSourceLearningProducesRetainablePattern(t *testing.T) {
	e := NewV8CognitiveEntity()
	for i := 0; i < 6; i++ {
		state, actions, correct := v12ProtectedState(12000+i*97, i%2, 3+(i%2))
		solved := false
		for step := 0; step < 12; step++ {
			action, err := e.ObserveAndAct(state, actions)
			if err != nil {
				t.Fatalf("episode=%d step=%d act error: %v", i, step, err)
			}
			ok := action == correct
			e.ObserveOutcome(state, action, state, func() float64 {
				if ok {
					return 1
				}
				return -1
			}(), ok)
			if ok {
				solved = true
				break
			}
		}
		if !solved {
			t.Fatalf("episode=%d could not reach evaluator target; valid=%t retained=%t nodes=%d edges=%d examples=%d expansions=%d key=%s",
				i, e.DirectedRepresentation.Valid, e.DirectedRepresentation.Retained,
				e.DirectedRepresentation.Nodes, len(e.DirectedRepresentation.Edges),
				len(e.DirectedRepresentation.Examples), e.DirectedRepresentation.SearchExpansions,
				e.DirectedRepresentation.Key())
		}
	}

	if !e.DirectedRepresentation.Valid {
		t.Fatalf("source learning left directed representation invalid: nodes=%d edges=%d examples=%d expansions=%d",
			e.DirectedRepresentation.Nodes, len(e.DirectedRepresentation.Edges),
			len(e.DirectedRepresentation.Examples), e.DirectedRepresentation.SearchExpansions)
	}
}
