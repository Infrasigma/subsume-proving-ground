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

type testT3KMS struct {
	private ed25519.PrivateKey
	signerID string
}

func (k testT3KMS) SignAbstractionHash(ctx context.Context, artifactHash, signerID string) (protocol.KMSSignedArtifact, error) {
	if err := ctx.Err(); err != nil {
		return protocol.KMSSignedArtifact{}, err
	}
	if signerID != k.signerID {
		return protocol.KMSSignedArtifact{}, context.Canceled
	}
	return protocol.SignAbstractionHash(artifactHash, k.signerID, k.private)
}

func TestSynthesizedProgramRejectsInvalidRegisterWithoutPanic(t *testing.T) {
	instructions := []SynthesizedInstruction{
		{Op: "input", A: maxSynthRegisters},
		{Op: "emit", A: 0},
		{Op: "halt"},
	}
	p := SynthesizedProgram{
		Version:       1,
		Language:      SynthesizedProgramLanguage,
		Source:        DisassembleSynthesizedProgram(instructions),
		Instructions:  instructions,
		Fuel:          10,
		MaxInputBytes: maxSynthInputBytes,
	}
	if err := p.Validate(); err == nil {
		t.Fatal("expected invalid register to be rejected")
	}
}

func TestDomainEscapeSynthesizesFromTrainingAndPassesHidden(t *testing.T) {
	task := DefaultT3DomainEscapeTasks()[0]
	program, err := SynthesizeDomainEscape(context.Background(), task)
	if err != nil {
		t.Fatalf("synthesis failed: %v", err)
	}
	if err := program.Validate(); err != nil {
		t.Fatalf("synthesized program invalid: %v", err)
	}
	if err := DefaultT3DomainEscapeVerifier().(StaticReactorVerifier).VerifySynthesizedProgram(context.Background(), task, program); err != nil {
		t.Fatalf("hidden verification failed: %v", err)
	}
	got, err := program.Execute(context.Background(), "autonomous reactor")
	if err != nil {
		t.Fatalf("sandbox execution failed: %v", err)
	}
	want := "AUtOnOmOUs rEActOr"
	if got != want {
		t.Fatalf("sandbox output = %q, want %q", got, want)
	}
}

func TestSynthesizedProgramFuelBoundsLoops(t *testing.T) {
	instructions := []SynthesizedInstruction{
		{Op: "const", A: 0, B: 0},
		{Op: "const", A: 1, B: 1},
		{Op: "add", A: 0, B: 0, C: 1},
		{Op: "jump", A: 2},
		{Op: "emit", A: 2},
		{Op: "halt"},
	}
	p := mustBuildSynthProgram(instructions)
	if _, err := p.Execute(context.Background(), ""); err == nil {
		t.Fatal("expected fuel exhaustion from nonterminating program")
	}
}

func TestT3DomainEscapeAdmissionAndRehydration(t *testing.T) {
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
	signerID := "test-t3-signer"
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	task := DefaultT3DomainEscapeTasks()[0]

	runtime := &AdaptiveAcquisitionRuntime{
		Abstractions:       AbstractionLibrary{},
		AbstractionKMS:     testT3KMS{private: private, signerID: signerID},
		AdmissionLedger:    store,
		KMSSignerID:        signerID,
		TrustedKMSPublicKeyB64: pubB64,
	}
	persisted, err := NewPersistentAbstractionLibrary(filepath.Join(dir, "abstractions.json"))
	if err != nil {
		t.Fatalf("create persistent library: %v", err)
	}
	queue := NewInMemoryReactorTaskQueue(task)
	reactor := &ContinuousReactor{
		Runtime:             runtime,
		Queue:               queue,
		Admissions:          store,
		PersistentLibrary:   persisted,
		TrustedSignerID:     signerID,
		TrustedPublicKeyB64: pubB64,
		Verifier:            DefaultT3DomainEscapeVerifier(),
		MaxTasks:            1,
	}
	results, err := reactor.Run(ctx)
	if err != nil {
		t.Fatalf("reactor run failed: %v", err)
	}
	if len(results) != 1 || !results[0].Solved {
		t.Fatalf("unexpected reactor result: %+v", results)
	}
	admitted, ok := runtime.Abstractions.Find(results[0].DiscoveredAbstractionID)
	if !ok {
		t.Fatalf("admitted program %q not found", results[0].DiscoveredAbstractionID)
	}
	if admitted.ArtifactType != SynthesizedProgramArtifactType || admitted.SynthesizedProgram == nil {
		t.Fatalf("unexpected admitted artifact: type=%q program=%v", admitted.ArtifactType, admitted.SynthesizedProgram != nil)
	}
	receipt, err := store.GetAbstractionAdmission(ctx, results[0].AdmissionRef)
	if err != nil {
		t.Fatalf("read admission: %v", err)
	}
	if receipt.ArtifactType != SynthesizedProgramArtifactType {
		t.Fatalf("receipt artifact type = %q", receipt.ArtifactType)
	}

	rehydrated, err := NewPersistentAbstractionLibrary(filepath.Join(dir, "abstractions.json"))
	if err != nil {
		t.Fatalf("open persisted library: %v", err)
	}
	dst := &AbstractionLibrary{}
	if err := rehydrated.RestoreWithTrustedAdmissions(ctx, dst, map[string]string{signerID: pubB64}, store); err != nil {
		t.Fatalf("rehydration failed: %v", err)
	}
	got, ok := dst.Find(admitted.ID)
	if !ok || got.SynthesizedProgram == nil || got.ArtifactType != SynthesizedProgramArtifactType {
		t.Fatalf("rehydrated program missing or altered: %+v", got)
	}
}
