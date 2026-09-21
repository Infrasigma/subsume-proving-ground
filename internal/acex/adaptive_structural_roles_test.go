package acex

import "testing"

func adaptiveRoleState(prefix string, surface int) (RelationalState, []string, string) {
	actions := []string{prefix+"-a", prefix+"-b", prefix+"-c"}
	correct := actions[1]
	actionKind, supportKind, edgeKind := "lever", "peg", "engage"
	if surface == 1 {
		actionKind, supportKind, edgeKind = "token", "port", "route"
	}
	nodes := make([]RelNode, 0, 20)
	edges := make([]RelEdge, 0, 24)
	for _, a := range actions {
		nodes = append(nodes, RelNode{ID:a, Kind:actionKind, Attrs:map[string]string{"surface": "opaque"}})
	}
	addNode := func(id string) {
		nodes = append(nodes, RelNode{ID:id, Kind:supportKind, Attrs:map[string]string{"surface":"opaque"}})
	}
	addEdge := func(from,to string) {
		edges = append(edges, RelEdge{From:from, To:to, Kind:edgeKind})
	}

	// Correct action: two immediate neighbors, sharing one radius-2 node.
	c1 := prefix+"-c1"
	c2 := prefix+"-c2"
	cShared := prefix+"-c-shared"
	addNode(c1)
	addNode(c2)
	addNode(cShared)
	addEdge(correct, c1)
	addEdge(correct, c2)
	addEdge(c1, cShared)
	addEdge(c2, cShared)

	// Distractors: identical radius-1 counts, but each neighbor reaches
	// its own separate radius-2 leaf. Radius 1 cannot distinguish these.
	for _, a := range []string{actions[0], actions[2]} {
		n1, n2 := a+"-n1", a+"-n2"
		l1, l2 := a+"-l1", a+"-l2"
		addNode(n1)
		addNode(n2)
		addNode(l1)
		addNode(l2)
		addEdge(a, n1)
		addEdge(a, n2)
		addEdge(n1, l1)
		addEdge(n2, l2)
	}

	return RelationalState{Nodes:nodes, Edges:edges}, actions, correct
}

func TestV8AdaptiveStructuralRoleInventionAndTransfer(t *testing.T) {
	learner := NewV8AdaptiveStructuralRoleLearner()
	source, sourceActions, sourceCorrect := adaptiveRoleState("src", 0)

	// Force a representation collision: a radius-1 false role first, then the
	// same shallow signature receives a verified positive outcome.
	wrong := sourceActions[0]
	learner.Observe(source, wrong, -1, false)
	learner.Observe(source, sourceCorrect, 1, false)

	if learner.ActiveRadius < 2 {
		t.Fatalf("representation did not expand after collision: radius=%d", learner.ActiveRadius)
	}
	if learner.InventedRepresentation() != "rooted-neighborhood-radius-2" {
		t.Fatalf("unexpected invented representation: %s", learner.InventedRepresentation())
	}

	target, targetActions, targetCorrect := adaptiveRoleState("dst", 1)
	if sourceCorrect == targetCorrect {
		t.Fatal("surface change reused action identity")
	}
	got, ok := learner.Select(target, targetActions)
	if !ok || got != targetCorrect {
		t.Fatalf("invented representation failed transfer: got=%q want=%q", got, targetCorrect)
	}
}
