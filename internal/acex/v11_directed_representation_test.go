package acex

import "testing"

func v11ChainState(prefix string, reverse bool, noise int) (RelationalState, []string, string) {
	actions := []string{prefix + "-a", prefix + "-b", prefix + "-c"}
	correct := actions[1]
	nodes := make([]RelNode, 0, 20)
	edges := make([]RelEdge, 0, 20)
	for _, action := range actions {
		nodes = append(nodes, RelNode{ID:action, Kind:"opaque-action", Attrs:map[string]string{"surface":"shifted"}})
		a := action + "-x"
		b := action + "-y"
		nodes = append(nodes,
			RelNode{ID:a, Kind:"opaque-node", Attrs:map[string]string{"noise":"1"}},
			RelNode{ID:b, Kind:"opaque-node", Attrs:map[string]string{"noise":"2"}},
		)
		if action == correct && !reverse {
			edges = append(edges,
				RelEdge{From:action, To:a, Kind:"r"},
				RelEdge{From:a, To:b, Kind:"r"},
			)
		} else {
			edges = append(edges,
				RelEdge{From:b, To:a, Kind:"r"},
				RelEdge{From:a, To:action, Kind:"r"},
			)
		}
	}
	for i := 0; i < noise; i++ {
		n := prefix + "-noise-" + string(rune('a'+i))
		nodes = append(nodes, RelNode{ID:n, Kind:"distractor", Attrs:map[string]string{"noise":"x"}})
	}
	return RelationalState{Nodes:nodes, Edges:edges}, actions, correct
}

func TestV11DirectedRepresentationInventionAndReload(t *testing.T) {
	r := NewV11DirectedExecutableRepresentation()
	for i := 0; i < 3; i++ {
		state, actions, correct := v11ChainState("src", false, i)
		r.Record(state, actions[(i+1)%len(actions)], -1, false)
		r.Record(state, correct, 1, false)
	}
	if !r.Synthesize() {
		t.Fatalf("directed representation synthesis failed: expansions=%d", r.SearchExpansions)
	}
	if len(r.Edges) < 2 || r.Nodes < 3 {
		t.Fatalf("synthesized representation is too weak: %+v", r)
	}
	state, actions, correct := v11ChainState("target", false, 4)
	got, ok := r.Select(state, actions)
	if !ok || got != correct {
		t.Fatalf("directed representation did not transfer: got=%q want=%q key=%s", got, correct, r.Key())
	}
	wrongState, wrongActions, wrongCorrect := v11ChainState("wrong", true, 4)
	got, ok = r.Select(wrongState, wrongActions)
	if ok && got == wrongCorrect {
		t.Fatalf("directed representation ignored edge direction: got=%q", got)
	}
	if err := r.ForgetExamples(); err != nil {
		t.Fatal(err)
	}
	artifact, err := r.Artifact()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadV11DirectedRepresentation(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Examples) != 0 || !loaded.Retained {
		t.Fatalf("artifact reload retained raw examples or lost retention: %+v", loaded)
	}
	got, ok = loaded.Select(state, actions)
	if !ok || got != correct {
		t.Fatalf("reloaded representation failed transfer: got=%q want=%q", got, correct)
	}
}

func TestV11ArtifactContainsNoRawEpisodes(t *testing.T) {
	r := NewV11DirectedExecutableRepresentation()
	state, actions, correct := v11ChainState("opaque-source", false, 0)
	r.Record(state, actions[0], -1, false)
	r.Record(state, correct, 1, false)
	if !r.Synthesize() {
		t.Fatal("synthesis failed")
	}
	if err := r.ForgetExamples(); err != nil {
		t.Fatal(err)
	}
	artifact, err := r.Artifact()
	if err != nil {
		t.Fatal(err)
	}
	if len(artifact) == 0 {
		t.Fatal("empty artifact")
	}
	if containsString(artifact, "opaque-source") || containsString(artifact, "examples") || containsString(artifact, "src") {
		t.Fatalf("artifact appears to contain raw episode material: %s", artifact)
	}
}
