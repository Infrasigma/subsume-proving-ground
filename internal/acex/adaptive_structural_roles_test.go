package acex

import "testing"

func adaptiveRoleState(prefix string, surface int) (RelationalState, []string, string) {
	actions := []string{prefix+"-a", prefix+"-b", prefix+"-c"}
	correct := actions[1]
	actionKind, supportKind, edgeKind := "lever", "peg", "engage"
	if surface == 1 {
		actionKind, supportKind, edgeKind = "token", "port", "route"
	}
	nodes := make([]RelNode, 0, 12)
	edges := make([]RelEdge, 0, 16)
	for _, a := range actions {
		nodes = append(nodes, RelNode{ID:a, Kind:actionKind, Attrs:map[string]string{"surface": "opaque"}})
	}
	addSupport := func(action, id string) {
		nodes = append(nodes, RelNode{ID:id, Kind:supportKind, Attrs:map[string]string{"surface": "opaque"}})
		edges = append(edges, RelEdge{From:action, To:id, Kind:edgeKind})
	}

	// Correct action: its two neighbors are adjacent, forming a triangle.
	c1 := prefix+"-c1"
	c2 := prefix+"-c2"
	addSupport(correct, c1)
	addSupport(correct, c2)
	edges = append(edges, RelEdge{From:c1, To:c2, Kind:edgeKind})

	// Distractors: same radius-1 degree statistics, but their two neighbors
	// extend outward instead of connecting to each other.
	for _, a := range []string{actions[0], actions[2]} {
		n1, n2 := a+"-n1", a+"-n2"
		l1, l2 := a+"-l1", a+"-l2"
		addSupport(a, n1)
		addSupport(a, n2)
		addSupport(n1, l1)
		addSupport(n2, l2)
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
