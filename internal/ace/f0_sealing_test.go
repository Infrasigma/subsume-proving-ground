package ace

import (
	"strings"
	"testing"
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
