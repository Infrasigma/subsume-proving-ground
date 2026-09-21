package acex

import (
	"context"
	"fmt"
	"testing"
)

func e1SourceState(prefix string) (RelationalState, []string) {
	actions := []string{prefix + "-a", prefix + "-b", prefix + "-c"}
	nodes := make([]RelNode, 0, 18)
	edges := make([]RelEdge, 0, 18)

	for _, action := range actions {
		for _, suffix := range []string{"x", "y", "z", "u", "v"} {
			nodes = append(nodes, RelNode{ID: action + "-" + suffix, Kind: "opaque"})
		}
		nodes = append(nodes, RelNode{ID: action, Kind: "opaque-action"})
	}

	// Negative candidates have an outgoing edge from the root but no directed
	// length-2 path beginning at the root.
	edges = append(edges,
		RelEdge{From: actions[0], To: actions[0] + "-x"},
		RelEdge{From: actions[0] + "-x", To: actions[0]},
		RelEdge{From: actions[0] + "-y", To: actions[0] + "-z"},
		RelEdge{From: actions[0] + "-u", To: actions[0] + "-v"},

		RelEdge{From: actions[2], To: actions[2] + "-x"},
		RelEdge{From: actions[2] + "-y", To: actions[2]},
		RelEdge{From: actions[2] + "-z", To: actions[2] + "-u"},
		RelEdge{From: actions[2] + "-v", To: actions[2] + "-x"},
	)

	// Positive candidate contains a directed chain plus additional topology.
	edges = append(edges,
		RelEdge{From: actions[1], To: actions[1] + "-x"},
		RelEdge{From: actions[1] + "-x", To: actions[1] + "-y"},
		RelEdge{From: actions[1] + "-y", To: actions[1] + "-z"},
		RelEdge{From: actions[1] + "-u", To: actions[1]},
		RelEdge{From: actions[1] + "-v", To: actions[1] + "-u"},
	)

	return RelationalState{Nodes: nodes, Edges: edges}, actions
}

func e1HoldoutState() (RelationalState, []string, string) {
	actions := []string{"hold-z", "hold-b", "hold-a"}
	nodes := make([]RelNode, 0, 18)
	edges := make([]RelEdge, 0, 24)

	for _, action := range actions {
		for _, suffix := range []string{"m", "n", "o", "p", "q"} {
			nodes = append(nodes, RelNode{ID: action + "-" + suffix, Kind: "shifted"})
		}
		nodes = append(nodes, RelNode{ID: action, Kind: "shifted-action"})
	}

	// Decoy 0: two-cycle from root, so a root->node edge exists but no
	// root-anchored directed chain of length two.
	edges = append(edges,
		RelEdge{From: actions[0], To: actions[0] + "-m"},
		RelEdge{From: actions[0] + "-m", To: actions[0]},
		RelEdge{From: actions[0] + "-n", To: actions[0] + "-o"},
		RelEdge{From: actions[0] + "-p", To: actions[0] + "-q"},
	)

	// Correct holdout: the same abstract chain is embedded in a different
	// topology, with an upstream tail and a longer forward chain.
	edges = append(edges,
		RelEdge{From: actions[1], To: actions[1] + "-m"},
		RelEdge{From: actions[1] + "-m", To: actions[1] + "-n"},
		RelEdge{From: actions[1] + "-n", To: actions[1] + "-o"},
		RelEdge{From: actions[1] + "-p", To: actions[1]},
		RelEdge{From: actions[1] + "-q", To: actions[1] + "-p"},
	)

	// Decoy 2: incoming chain toward root; it is directionally distinct.
	edges = append(edges,
		RelEdge{From: actions[2] + "-m", To: actions[2]},
		RelEdge{From: actions[2] + "-n", To: actions[2] + "-m"},
		RelEdge{From: actions[2] + "-o", To: actions[2] + "-n"},
		RelEdge{From: actions[2] + "-p", To: actions[2] + "-q"},
	)

	return RelationalState{Nodes: nodes, Edges: edges}, actions, actions[1]
}

func TestV12E1HermeticGeneralityProof(t *testing.T) {
	source, sourceActions := e1SourceState("src")
	entity := NewV8CognitiveEntity()

	// Train the existing V11 bounded enumerative learner. This test deliberately
	// does not call it CEGIS: no CEGIS implementation exists in V12 yet.
	entity.DirectedRepresentation.Record(source, sourceActions[0], -1, false)
	entity.DirectedRepresentation.Record(source, sourceActions[1], 1, true)
	entity.DirectedRepresentation.Record(source, sourceActions[2], -1, false)
	if !entity.DirectedRepresentation.Synthesize() {
		t.Fatalf("source synthesis failed: expansions=%d", entity.DirectedRepresentation.SearchExpansions)
	}
	if !entity.DirectedRepresentation.Retained {
		if err := entity.DirectedRepresentation.ForgetExamples(); err != nil {
			t.Fatal(err)
		}
	}

	artifact, err := entity.ExportRetainedRepresentation()
	if err != nil {
		t.Fatal(err)
	}
	pattern := entity.DirectedRepresentation.Key()
	artifactBytes := len(entity.HermeticArtifact)
	artifactHash := entity.HermeticArtifactHash

	// Destructive boundary simulation: instantiate a fresh entity from only the
	// exported artifact, then poison the Go representation with a conflicting
	// two-cycle. A Go DirectedRepresentation.Select implementation would choose
	// hold-z for the holdout; only the Wasm artifact should select hold-b.
	rehydrated := NewV8CognitiveEntity()
	if err := rehydrated.LoadRetainedRepresentation(artifact); err != nil {
		t.Fatal(err)
	}
	rehydrated.DirectedRepresentation.Nodes = 3
	rehydrated.DirectedRepresentation.Edges = []V11DirectedPatternEdge{
		{From: 0, To: 1},
		{From: 1, To: 0},
	}
	rehydrated.DirectedRepresentation.Valid = true
	rehydrated.DirectedRepresentation.Retained = true

	holdout, holdoutActions, correct := e1HoldoutState()
	got, err := rehydrated.ObserveAndAct(holdout, holdoutActions)
	if err != nil {
		t.Fatalf("hermetic holdout decision failed: %v", err)
	}
	if got != correct {
		t.Fatalf("E1 holdout failed: got=%q want=%q", got, correct)
	}

	allocs := testing.AllocsPerRun(100, func() {
		got, err := rehydrated.decideHermetic(holdout, holdoutActions)
		if err != nil {
			t.Fatalf("warm hermetic inference failed: %v", err)
		}
		if got != correct {
			t.Fatalf("warm hermetic inference chose %q, want %q", got, correct)
		}
	})
	if allocs != 0 {
		t.Fatalf("warm hermetic decision allocated %.2f Go objects/call", allocs)
	}

	t.Logf("E1_SOURCE_EXPANSIONS=%d", entity.DirectedRepresentation.SearchExpansions)
	t.Logf("E1_LEARNED_PATTERN=%s", pattern)
	t.Logf("E1_ARTIFACT_BYTES=%d", artifactBytes)
	t.Logf("E1_ARTIFACT_SHA256=%s", artifactHash)
	t.Logf("E1_HOLDOUT_ACTION=%s", got)
	t.Logf("E1_HOLDOUT_ACTION_INDEX=%d", 1)
	t.Logf("E1_ABI_CANDIDATE_NODES=16")
	t.Logf("E1_WARM_INFERENCE_ALLOCS=%.2f", allocs)

	executor, err := NewV12WasmDecisionExecutor(context.Background(), rehydrated.HermeticArtifact)
	if err != nil {
		t.Fatal(err)
	}
	defer executor.Close(context.Background())
	t.Logf("E1_WASM_DECIDE_ABI=%s", fmt.Sprintf("(ptr,count)->index"))
}
