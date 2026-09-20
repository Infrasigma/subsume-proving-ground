package ace

import (
	"context"
	"testing"
)

func TestCognitiveProbeCompositionExceedsDirectFrontier(t *testing.T) {
	ctx := context.Background()

	result, err := RunCognitiveProbe(ctx)
	if err != nil {
		t.Fatalf("cognitive probe failed: %v", err)
	}

	if !result.PrimitiveAHiddenVerified {
		t.Fatal("primitive A was not independently verified on held-out cases")
	}
	if !result.PrimitiveBHiddenVerified {
		t.Fatal("primitive B was not independently verified on held-out cases")
	}
	if result.DirectTargetSolved {
		t.Fatal("direct synthesis solved the target; the composition boundary is not strict")
	}
	if !result.ComposedTargetSolved {
		t.Fatal("composed capability did not solve the held-out target")
	}
	if result.CompositionDepth != 2 {
		t.Fatalf("unexpected composition depth: got %d want 2", result.CompositionDepth)
	}
}
