package ace

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/Infrasigma/subsume-proving-ground/internal/ledger"
	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
)

func TestT4MetacognitiveHotSwapSealsAndRehydrates(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := ledger.Open(filepath.Join(dir, "admissions.db"))
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	defer store.Close()

	pub, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate signer: %v", err)
	}
	signerID := "test-t4-signer"
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	runtime := &AdaptiveAcquisitionRuntime{
		Abstractions:            AbstractionLibrary{},
		AbstractionKMS:          testT3KMS{private: private, signerID: signerID},
		AdmissionLedger:         store,
		KMSSignerID:             signerID,
		TrustedKMSPublicKeyB64:  pubB64,
	}
	persistent, err := NewPersistentAbstractionLibrary(filepath.Join(dir, "abstractions.json"))
	if err != nil {
		t.Fatalf("create persistent library: %v", err)
	}

	tasks := DefaultT4MetacognitiveTasks()
	reactor := &ContinuousReactor{
		Runtime:             runtime,
		Queue:               NewInMemoryReactorTaskQueue(tasks...),
		Admissions:          store,
		PersistentLibrary:   persistent,
		TrustedSignerID:     signerID,
		TrustedPublicKeyB64: pubB64,
		Verifier:            DefaultT3DomainEscapeVerifier(),
		MaxTasks:            2,
	}
	results, err := reactor.Run(ctx)
	if err != nil {
		t.Fatalf("reactor run failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected meta-task + immediate post-swap task, got %d results", len(results))
	}
	if !results[0].Solved {
		t.Fatalf("meta-task failed: %+v", results[0])
	}
	if !results[1].Solved {
		t.Fatalf("post-hot-swap task failed: %+v", results[1])
	}
	if results[0].EvaluatedCandidates != 1 {
		t.Fatalf("meta candidate did not produce a one-iteration improvement: %+v", results[0])
	}
	if results[1].EvaluatedCandidates != 1 {
		t.Fatalf("next task did not use the hot-swapped heuristic: %+v", results[1])
	}
	if runtime.ActiveSearchHeuristic == nil {
		t.Fatal("runtime has no active search heuristic after metacognitive task")
	}
	if runtime.ActiveSearchHeuristic.StringStrategy != "prefer-uppercase-vowels" {
		t.Fatalf("unexpected active string heuristic %q", runtime.ActiveSearchHeuristic.StringStrategy)
	}

	receipt, err := store.GetAbstractionAdmission(ctx, results[0].AdmissionRef)
	if err != nil {
		t.Fatalf("read metacognitive admission: %v", err)
	}
	if receipt.ArtifactType != SearchHeuristicArtifactType {
		t.Fatalf("metacognitive admission type = %q", receipt.ArtifactType)
	}
	if err := protocol.VerifyAbstractionAdmissionReceipt(receipt, pubB64); err != nil {
		t.Fatalf("metacognitive admission receipt invalid: %v", err)
	}

	var heuristicCount int
	for _, a := range runtime.Abstractions.Abstractions {
		if a.ArtifactType == SearchHeuristicArtifactType {
			heuristicCount++
			if a.SearchHeuristic == nil {
				t.Fatal("installed search heuristic has no payload")
			}
			if a.SearchHeuristic.StringStrategy == runtime.ActiveSearchHeuristic.StringStrategy &&
				a.ID != results[0].DiscoveredAbstractionID {
				t.Fatalf("active heuristic points at an unexpected artifact %q", a.ID)
			}
		}
	}
	if heuristicCount < 2 {
		t.Fatalf("expected bootstrap + evolved search heuristic, got %d", heuristicCount)
	}

	runtime2 := &AdaptiveAcquisitionRuntime{
		Abstractions:            AbstractionLibrary{},
		AbstractionKMS:          testT3KMS{private: private, signerID: signerID},
		AdmissionLedger:         store,
		KMSSignerID:             signerID,
		TrustedKMSPublicKeyB64:  pubB64,
	}
	persistent2, err := NewPersistentAbstractionLibrary(filepath.Join(dir, "abstractions.json"))
	if err != nil {
		t.Fatalf("reopen persistent library: %v", err)
	}
	reactor2 := &ContinuousReactor{
		Runtime:             runtime2,
		Queue:               NewInMemoryReactorTaskQueue(),
		Admissions:          store,
		PersistentLibrary:   persistent2,
		TrustedSignerID:     signerID,
		TrustedPublicKeyB64: pubB64,
		Verifier:            DefaultT3DomainEscapeVerifier(),
	}
	if err := reactor2.Hydrate(ctx); err != nil {
		t.Fatalf("rehydration failed: %v", err)
	}
	if runtime2.ActiveSearchHeuristic == nil {
		t.Fatal("rehydrated runtime has no active search heuristic")
	}
	if runtime2.ActiveSearchHeuristic.StringStrategy != runtime.ActiveSearchHeuristic.StringStrategy {
		t.Fatalf("rehydrated heuristic %q differs from active %q", runtime2.ActiveSearchHeuristic.StringStrategy, runtime.ActiveSearchHeuristic.StringStrategy)
	}
}

func TestT4SearchHeuristicPreservesCandidateSet(t *testing.T) {
	names := []string{"identity", "reverse", "upper", "lower", "upper-vowels", "lower-vowels"}
	want := map[string]int{}
	for _, name := range names {
		want[name]++
	}
	for _, heuristic := range append([]SearchHeuristicProgram{DefaultSearchHeuristicProgram()}, EnumerateSearchHeuristicPrograms()...) {
		ordered, err := orderStringCandidateNames(names, &heuristic)
		if err != nil {
			t.Fatalf("heuristic %q rejected: %v", heuristic.StringStrategy, err)
		}
		if len(ordered) != len(names) {
			t.Fatalf("heuristic %q changed candidate count: got %d want %d", heuristic.StringStrategy, len(ordered), len(names))
		}
		got := map[string]int{}
		for _, name := range ordered {
			got[name]++
		}
		for name, count := range want {
			if got[name] != count {
				t.Fatalf("heuristic %q dropped or duplicated candidate %q: got %d want %d", heuristic.StringStrategy, name, got[name], count)
			}
		}
	}
}
