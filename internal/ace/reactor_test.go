package ace

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/Infrasigma/subsume-proving-ground/internal/ledger"
)

func newReactorTestFixture(t *testing.T) (*ContinuousReactor, *ledger.Ledger, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	store, err := ledger.Open(filepath.Join(t.TempDir(), "admission.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	signerID := "reactor-test-kms"
	trusted := base64.StdEncoding.EncodeToString(pub)
	runtime := &AdaptiveAcquisitionRuntime{
		AbstractionKMS:         testAbstractionKMS{signerID: signerID, privateKey: priv},
		AdmissionLedger:        store,
		KMSSignerID:            signerID,
		TrustedKMSPublicKeyB64: trusted,
		Abstractions:           AbstractionLibrary{},
	}
	persistent, err := NewPersistentAbstractionLibrary(filepath.Join(t.TempDir(), "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	queue := NewInMemoryReactorTaskQueue(DefaultT2ReactorTasks()...)
	reactor := &ContinuousReactor{
		Runtime:             runtime,
		Queue:               queue,
		Admissions:          store,
		PersistentLibrary:   persistent,
		TrustedSignerID:     signerID,
		TrustedPublicKeyB64: trusted,
		Verifier:            DefaultT2ReactorVerifier(),
	}
	return reactor, store, trusted
}

func TestContinuousReactorCompoundsAcrossTasks(t *testing.T) {
	reactor, store, trusted := newReactorTestFixture(t)
	results, err := reactor.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d reactor results want 2", len(results))
	}
	if !results[0].Solved || results[0].DiscoveredAbstractionID == "" || results[0].AdmissionRef == "" {
		t.Fatalf("first task did not produce a sealed abstraction: %#v", results[0])
	}
	if !results[1].Solved {
		t.Fatalf("second task failed: %#v", results[1])
	}
	if results[1].UsedAbstractionID != results[0].DiscoveredAbstractionID {
		t.Fatalf("second task used %q want transferred abstraction %q", results[1].UsedAbstractionID, results[0].DiscoveredAbstractionID)
	}
	receipt, err := store.GetAbstractionAdmission(context.Background(), results[0].AdmissionRef)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ArtifactHash == "" || receipt.SignerID == "" {
		t.Fatalf("ledger receipt incomplete: %#v", receipt)
	}

	freshPersistent, err := NewPersistentAbstractionLibrary(filepath.Join(t.TempDir(), "rehydrated.json"))
	if err != nil {
		t.Fatal(err)
	}
	freshPersistent.Data = reactor.PersistentLibrary.Data
	freshLib := AbstractionLibrary{}
	if err := freshPersistent.RestoreWithTrustedAdmissions(
		context.Background(),
		&freshLib,
		map[string]string{reactor.TrustedSignerID: trusted},
		store,
	); err != nil {
		t.Fatal(err)
	}
	if _, ok := freshLib.Find(results[0].DiscoveredAbstractionID); !ok {
		t.Fatalf("rehydrated library missing %s", results[0].DiscoveredAbstractionID)
	}
}

func TestContinuousReactorRejectsForgedPersistenceDuringHydration(t *testing.T) {
	reactor, store, trusted := newReactorTestFixture(t)
	results, err := reactor.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || !results[0].Solved {
		t.Fatalf("expected successful baseline run: %#v", results)
	}
	forged, err := NewPersistentAbstractionLibrary(filepath.Join(t.TempDir(), "forged.json"))
	if err != nil {
		t.Fatal(err)
	}
	forged.Data = reactor.PersistentLibrary.Data
	forged.Data.Abstractions = append([]AcquiredAbstraction(nil), forged.Data.Abstractions...)
	forged.Data.Abstractions[0].LedgerAdmissionRef = "forged-ledger-ref"
	fresh := AbstractionLibrary{}
	err = forged.RestoreWithTrustedAdmissions(
		context.Background(),
		&fresh,
		map[string]string{reactor.TrustedSignerID: trusted},
		store,
	)
	if err == nil {
		t.Fatal("forged persisted ledger admission was accepted")
	}
}

func TestContinuousReactorContinuesAfterSearchFailure(t *testing.T) {
	reactor, _, trusted := newReactorTestFixture(t)
	bad := DefaultT2ReactorTasks()[0]
	bad.ID = "00-bad-search"
	bad.MaxSearchDepth = 1
	bad.MinProcedureSteps = 1
	bad.AdmitAsAbstraction = false
	bad.RequireAbstractionID = "does-not-exist"
	next := DefaultT2ReactorTasks()[0]
	results, err := (&ContinuousReactor{
		Runtime:             reactor.Runtime,
		Queue:               NewInMemoryReactorTaskQueue(bad, next),
		Admissions:          reactor.Admissions,
		PersistentLibrary:   reactor.PersistentLibrary,
		TrustedSignerID:     reactor.TrustedSignerID,
		TrustedPublicKeyB64: trusted,
		Verifier:            DefaultT2ReactorVerifier(),
	}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results want 2", len(results))
	}
	if results[0].Solved || results[0].Error == "" {
		t.Fatalf("search failure task did not fail cleanly: %#v", results[0])
	}
	if !results[1].Solved {
		t.Fatalf("reactor did not continue to next task: %#v", results[1])
	}
}

func TestReactorTaskValidationRejectsMalformedExamples(t *testing.T) {
	task := DefaultT2ReactorTasks()[0]
	task.Examples[0].Expected = nil
	if err := task.Validate(); err == nil {
		t.Fatal("malformed reactor task passed validation")
	}
}
