package ace

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
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
	signed, err := protocol.SignAbstractionHash(hash, "test-kms", priv)
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
