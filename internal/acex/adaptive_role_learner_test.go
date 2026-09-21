package acex

import "testing"

func adaptiveTriangleState(domain int) (RelationalState, []string, string) {
	prefix := "a"
	if domain == 1 {
		prefix = "z"
	}
	actions := []string{prefix + "-one", prefix + "-two", prefix + "-three"}
	correct := actions[1]

	nodes := []RelNode{}
	edges := []RelEdge{}
	add := func(id string) {
		nodes = append(nodes, RelNode{ID:id, Kind:"surface-" + prefix, Attrs:map[string]string{"opaque":prefix}})
	}
	for _, action := range actions {
		add(action)
	}
	for _, action := range []string{actions[0], actions[2]} {
		n1, n2 := action+"-n1", action+"-n2"
		l1, l2 := action+"-l1", action+"-l2"
		for _, id := range []string{n1, n2, l1, l2} { add(id) }
		edges = append(edges,
			RelEdge{From:action, To:n1, Kind:"surface-edge"},
			RelEdge{From:action, To:n2, Kind:"surface-edge"},
			RelEdge{From:n1, To:l1, Kind:"surface-edge"},
			RelEdge{From:n2, To:l2, Kind:"surface-edge"},
		)
	}
	c1, c2, shared := correct+"-n1", correct+"-n2", correct+"-shared"
	for _, id := range []string{c1, c2, shared} { add(id) }
	edges = append(edges,
		RelEdge{From:correct, To:c1, Kind:"surface-edge"},
		RelEdge{From:correct, To:c2, Kind:"surface-edge"},
		RelEdge{From:c1, To:shared, Kind:"surface-edge"},
		RelEdge{From:c2, To:shared, Kind:"surface-edge"},
	)
	return RelationalState{Nodes:nodes, Edges:edges}, actions, correct
}

func TestAdaptiveRoleLearnerExpandsRadiusAndTransfers(t *testing.T) {
	state0, actions0, correct0 := adaptiveTriangleState(0)
	learner := NewAdaptiveRoleLearner()
	learner.Observe(state0, actions0[0], false)
	learner.Observe(state0, actions0[2], false)
	learner.Observe(state0, correct0, true)

	if !learner.Ready || learner.Radius != 2 {
		t.Fatalf("expected radius-2 representation after radius-1 failure: ready=%v radius=%d expansions=%d", learner.Ready, learner.Radius, learner.SearchExpansions)
	}
	state1, actions1, correct1 := adaptiveTriangleState(1)
	got, ok := learner.Predict(state1, actions1)
	if !ok || got != correct1 {
		t.Fatalf("transfer failed: got=%q want=%q ok=%v", got, correct1, ok)
	}
}
