package acex

import "testing"

func roleTransferState(prefix string, source bool) (RelationalState, []string, string) {
	actions := []string{prefix+"-a", prefix+"-b", prefix+"-c"}
	correct := actions[0]
	kindAction, kindSupport, edgeKind := "lever", "peg", "engage"
	if !source {
		kindAction, kindSupport, edgeKind = "token", "port", "routes"
	}
	nodes := []RelNode{
		{ID: correct, Kind: kindAction, Attrs: map[string]string{"surface": "changed"}},
		{ID: actions[1], Kind: kindAction, Attrs: map[string]string{"surface": "changed2"}},
		{ID: actions[2], Kind: kindAction, Attrs: map[string]string{"surface": "changed3"}},
	}
	for i := 0; i < 2; i++ {
		nodes = append(nodes, RelNode{
			ID: prefix+"-c-neighbor-"+string(rune('a'+i)),
			Kind: kindSupport,
			Attrs: map[string]string{"token": "opaque"},
		})
	}
	nodes = append(nodes,
		RelNode{ID: prefix+"-b-neighbor", Kind: kindSupport, Attrs: map[string]string{"token": "opaque"}},
		RelNode{ID: prefix+"-c-neighbor", Kind: kindSupport, Attrs: map[string]string{"token": "opaque"}},
		RelNode{ID: prefix+"-hub", Kind: kindSupport, Attrs: map[string]string{"token": "hub"}},
	)
	edges := []RelEdge{
		{From: correct, To: prefix+"-c-neighbor-a", Kind: edgeKind},
		{From: correct, To: prefix+"-c-neighbor-b", Kind: edgeKind},
		{From: prefix+"-c-neighbor-a", To: prefix+"-hub", Kind: edgeKind},
		{From: prefix+"-c-neighbor-b", To: prefix+"-hub", Kind: edgeKind},
		{From: prefix+"-b", To: prefix+"-b-neighbor", Kind: edgeKind},
		{From: prefix+"-c", To: prefix+"-c-neighbor", Kind: edgeKind},
	}
	return RelationalState{Nodes: nodes, Edges: edges}, actions, correct
}

func TestV8StructuralRoleTransfersAcrossSurfaceChange(t *testing.T) {
	learner := NewV8StructuralRoleLearner()
	source, sourceActions, sourceCorrect := roleTransferState("src", true)
	if _, ok := StructuralActionRoleKey(source, sourceCorrect); !ok {
		t.Fatal("source role was not representable")
	}
	learner.Observe(source, sourceCorrect, 1, false)

	target, targetActions, targetCorrect := roleTransferState("dst", false)
	if targetCorrect == sourceCorrect {
		t.Fatal("test accidentally reused action identity")
	}
	keySource, _ := StructuralActionRoleKey(source, sourceCorrect)
	keyTarget, _ := StructuralActionRoleKey(target, targetCorrect)
	if keySource != keyTarget {
		t.Fatalf("surface-invariant role changed: %q vs %q", keySource, keyTarget)
	}
	got, ok := learner.Select(target, targetActions)
	if !ok || got != targetCorrect {
		t.Fatalf("structural role did not transfer: got=%q want=%q", got, targetCorrect)
	}
	if len(sourceActions) != len(targetActions) {
		t.Fatal("source/target action arity mismatch")
	}
}
