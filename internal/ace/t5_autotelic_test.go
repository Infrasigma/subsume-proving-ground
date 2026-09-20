package ace

import (
	"context"
	"testing"

	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
)

func TestT5AutotelicExpansionFromEmptyQueue(t *testing.T) {
	ctx := context.Background()
	reactor, store, trusted := newReactorTestFixture(t)
	reactor.Queue = NewInMemoryReactorTaskQueue()
	reactor.Verifier = DefaultT3DomainEscapeVerifier()
	reactor.AutotelicGenerator = DefaultAutotelicTaskGenerator()
	reactor.MaxAutotelicTasks = 1
	reactor.MaxTasks = 1

	if err := reactor.Hydrate(ctx); err != nil {
		t.Fatalf("initial hydration failed: %v", err)
	}
	if reactor.Runtime.ActiveSearchHeuristic == nil {
		t.Fatal("T4 active search heuristic was not hydrated")
	}
	bundle, err := reactor.AutotelicGenerator.Generate(ctx, &reactor.Runtime.Abstractions, reactor.Runtime.ActiveSearchHeuristic)
	if err != nil {
		t.Fatalf("autotelic generator failed: %v", err)
	}
	if !bundle.Task.Autotelic {
		t.Fatal("generated task was not marked autotelic")
	}
	if bundle.BoundaryScore != 1 || bundle.Complexity != 2 {
		t.Fatalf("unexpected complexity transition: boundary=%d complexity=%d", bundle.BoundaryScore, bundle.Complexity)
	}
	for _, hidden := range bundle.hidden {
		for _, public := range bundle.Task.Examples {
			if hidden.Input[0] == public.Input[0] || hidden.Expected[0] == public.Expected[0] {
				t.Fatalf("hidden fixture leaked into public training examples: hidden=%+v public=%+v", hidden, public)
			}
		}
	}
	if len(bundle.hidden) != 3 {
		t.Fatalf("expected three evaluator-owned hidden fixtures, got %d", len(bundle.hidden))
	}

	results, err := reactor.Run(ctx)
	if err != nil {
		t.Fatalf("empty-queue autotelic run failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected exactly one autonomous result, got %d", len(results))
	}
	result := results[0]
	if !result.Solved || !result.Autotelic {
		t.Fatalf("autotelic task did not solve cleanly: %+v", result)
	}
	if result.TaskID != AutotelicTaskID {
		t.Fatalf("unexpected generated task %q", result.TaskID)
	}
	if result.EvaluatedCandidates < 2 {
		t.Fatalf("generated task did not exercise a non-trivial bounded synthesis frontier: %d", result.EvaluatedCandidates)
	}
	if result.DiscoveredAbstractionID == "" || result.AdmissionRef == "" {
		t.Fatalf("autotelic capability was not durably admitted: %+v", result)
	}

	receipt, err := store.GetAbstractionAdmission(ctx, result.AdmissionRef)
	if err != nil {
		t.Fatalf("read autotelic admission: %v", err)
	}
	if receipt.ArtifactType != SynthesizedProgramArtifactType {
		t.Fatalf("autotelic admission artifact type = %q", receipt.ArtifactType)
	}
	if err := protocol.VerifyAbstractionAdmissionReceipt(receipt, trusted); err != nil {
		t.Fatalf("autotelic admission receipt failed cryptographic verification: %v", err)
	}

	admitted, ok := reactor.Runtime.Abstractions.Find(result.DiscoveredAbstractionID)
	if !ok {
		t.Fatalf("autotelic capability %q missing from runtime library", result.DiscoveredAbstractionID)
	}
	if admitted.Name != "autotelic:"+AutotelicGapStringReverseUpperVowels {
		t.Fatalf("unexpected autotelic artifact name %q", admitted.Name)
	}
	if admitted.SynthesizedProgram == nil {
		t.Fatal("autotelic admission has no synthesized program payload")
	}

	second, err := reactor.Run(ctx)
	if err != nil {
		t.Fatalf("second empty-queue run failed: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("autotelic generator repeated an already-admitted gap: %+v", second)
	}
}
