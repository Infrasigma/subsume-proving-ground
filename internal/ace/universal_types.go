package ace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type ResourceVector struct {
	Search  int
	Verify  int
	Memory  int
	Storage int
}

type Task struct {
	ID     string
	Goal   string
	Budget ResourceVector
}

type CapabilitySpecification struct {
	ID                   string
	DesiredBehaviour     string
	Inputs               []string
	Outputs              []string
	Invariants            []string
	AcceptanceTests       []string
	ResourceLimits        ResourceVector
	FailureCriteria       []string
	RegressionConstraints []string
	Provenance             string
	KnownExamples          []ProgramTestCase
}

type ArchitectureCandidate struct {
	ID              string
	Mechanism       string
	Interfaces      []string
	Advantage       string
	Assumptions     string
	Resources       ResourceVector
	Tests           []string
	RegressionRisks []string
	Provenance      string
}

type ModificationProposal struct {
	ID         string
	Capability CapabilitySpecification
	Candidate  ArchitectureCandidate
	Artifact   string
	Provenance string
}

func Hash(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("ace hash serialization failed: %v", err))
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func Prov(parts ...any) string {
	return Hash(append([]any{"provenance"}, parts...))
}

// ProgramFitsForTests exposes the concrete execution predicate to the
// cognitive integration package without exposing evaluator policy.
func ProgramFitsForTests(p UniversalProgram, cases []ProgramTestCase) bool {
	return programFits(p, cases)
}
