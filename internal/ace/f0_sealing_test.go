package ace

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"strings"
	"testing"
	
	"github.com/Infrasigma/subsume-proving-ground/internal/ledger"
	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
)

func TestExecutionRejectsManuallyInjectedUnsignedAbstraction(t *testing.T) {
	unsigned := AcquiredAbstraction{
		ID:   "unsigned-manual-abstraction",
		Name: "unsigned-manual-abstraction",
		Procedure: AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{
			{Op: "reverse"},
			{Op: "rotate", Arg: 1},
		}},
		Verification: VerificationResult{Status: "verified", Independent: true},
		Evidence: []AbstractionEvidence{{TaskStructure: "manual-injection", Verified: true, HeldOut: true}},
	}
	lib := AbstractionLibrary{
		Abstractions:  []AcquiredAbstraction{unsigned},
		TrustedSigners: map[string]string{"trusted-kms": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},
	}
	caller := AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{{Op: "call", Ref: unsigned.ID}}}
	stream := []ArchitectureCandidate{{Mechanism: "a"}, {Mechanism: "b"}}

	_, err := executeSearchProcedureWithLibrary(caller, stream, &lib)
	if err == nil {
		t.Fatal("unsigned manually injected abstraction executed")
	}
	if !strings.Contains(err.Error(), "cryptographic abstraction admission rejected") {
		t.Fatalf("execution failed for the wrong reason: %v", err)
	}
}


func testAdmitAbstraction(t *testing.T, a AcquiredAbstraction) AcquiredAbstraction {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil { t.Fatal(err) }
	_ = pub
	hash, _, err := a.canonicalArtifact()
	if err != nil { t.Fatal(err) }
	signerID := "test-kms-" + hash[:8]
	signed, err := protocol.SignAbstractionHash(hash, signerID, priv)
	if err != nil { t.Fatal(err) }
	store, err := ledger.Open(t.TempDir() + "/admission.db")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func(){ _ = store.Close() })
	receipt, err := store.AppendAbstractionAdmission(context.Background(), protocol.AbstractionAdmissionReceipt{KMSSignedArtifact:signed})
	if err != nil { t.Fatal(err) }
	a.ArtifactHash = hash
	a.KMSSignature = receipt.KMSSignedArtifact
	a.LedgerAdmissionRef = receipt.LedgerAdmissionRef
	a.LedgerAdmissionHash = receipt.LedgerAdmissionHash
	a.LedgerPreviousAdmissionHash = receipt.PreviousAdmissionHash
	a.LedgerCreatedAtUnix = receipt.CreatedAtUnix
	return a
}


type testAbstractionKMS struct { signerID string; privateKey ed25519.PrivateKey }

func (k testAbstractionKMS) SignAbstractionHash(ctx context.Context, artifactHash, signerID string) (protocol.KMSSignedArtifact, error) {
	if signerID != k.signerID { return protocol.KMSSignedArtifact{}, fmt.Errorf("unexpected test signer %q", signerID) }
	return protocol.SignAbstractionHash(artifactHash, signerID, k.privateKey)
}

type testAdmissionLedger struct { store *ledger.Ledger }

func (l *testAdmissionLedger) AppendAbstractionAdmission(ctx context.Context, r protocol.AbstractionAdmissionReceipt) (protocol.AbstractionAdmissionReceipt, error) {
	return l.store.AppendAbstractionAdmission(ctx, r)
}

func newF0TestRuntime(t *testing.T) AdaptiveAcquisitionRuntime {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil { t.Fatal(err) }
	store, err := ledger.Open(t.TempDir() + "/f0-admissions.db")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func(){ _ = store.Close() })
	const signerID = "test-runtime-kms"
	return AdaptiveAcquisitionRuntime{
		AbstractionKMS: testAbstractionKMS{signerID: signerID, privateKey: priv},
		AdmissionLedger: &testAdmissionLedger{store: store},
		KMSSignerID: signerID,
	}
}
