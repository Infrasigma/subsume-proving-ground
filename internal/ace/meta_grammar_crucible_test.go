package ace

import (
	"context"
	"testing"
)

// This test fails closed when no external meta-synthesis provider is configured.
// It admits a mutation only after generated Go compiles and solves evaluator-held hidden cases.
func TestAutonomousMetaGrammarMutation(t *testing.T) {
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
	if mutation.NodeKind == "recursive-stack-machine" {
		t.Fatalf("human-authored escape hatch leaked into mutation output")
	}
	if mutation.GeneratedSource == "" || !mutation.Compiled || !mutation.HiddenVerified {
		t.Fatalf("expected dynamically generated, compiled, hidden-verified node: kind=%q compiled=%v hidden_verified=%v", mutation.NodeKind, mutation.Compiled, mutation.HiddenVerified)
	}
	t.Logf("AUTONOMOUS_META_GRAMMAR_MUTATION diagnosis=%s confidence=%.2f node=%q delta=%v compiled=%v hidden_verified=%v source=%s trace=%q",
		diagnosis.Class, diagnosis.Confidence, mutation.NodeKind, mutation.GrammarDelta,
		mutation.Compiled, mutation.HiddenVerified, mutation.GeneratedSource, mutation.EvaluationTrace)
}
