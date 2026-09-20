package ace

import (
	"context"
	"testing"
)

func TestMetaGrammarCrucibleInitialFailure(t *testing.T) {
	ctx := context.Background()
	diagnosis, mutation, err := RunMetaGrammarCrucible(ctx, 20260921, 8)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("META_GRAMMAR_CRUCIBLE diagnosis=%s confidence=%.2f reason=%q evidence=%v mutation=%q delta=%v",
		diagnosis.Class, diagnosis.Confidence, diagnosis.Reason, diagnosis.Evidence,
		mutation.NodeKind, mutation.GrammarDelta)
	if diagnosis.Class != BottleneckSearchSpace {
		t.Fatalf("expected sealed grammar to exhaust the candidate frontier, got %s: %s", diagnosis.Class, diagnosis.Reason)
	}
	if mutation.NodeKind != "recursive-stack-machine" {
		t.Fatalf("expected recursive/stateful AST escape proposal, got %q", mutation.NodeKind)
	}
}
