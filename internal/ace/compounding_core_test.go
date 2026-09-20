package ace

import (
	"strings"
	"testing"
)

func TestRecursiveCapabilityCompoundingCore(t *testing.T) {
	previousSynthesisBudget := AdaptiveUniversalSynthesisMaxExpansions
	AdaptiveUniversalSynthesisMaxExpansions = 500_000
	t.Cleanup(func() { AdaptiveUniversalSynthesisMaxExpansions = previousSynthesisBudget })
	task, train, _, err := DeepCompositionFamily{}.Generate(1, false)
	if err != nil {
		t.Fatal(err)
	}
	hiddenTask, hidden, _, err := DeepCompositionFamily{}.Generate(1, true)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := GeneralCapabilitySpecification(task, train)
	if err != nil {
		t.Fatal(err)
	}
	hiddenSpec, err := GeneralCapabilitySpecification(hiddenTask, hidden)
	if err != nil {
		t.Fatal(err)
	}
	telemetry := AcquisitionTelemetry{
		TaskID:              "recursive-core",
		TaskStructure:       []string{"scalar", "composition", "depth-3"},
		KnownExamples:       len(train),
		CandidateCount:      3,
		CandidateFailures:   []string{"single-primitive curriculum control"},
		Counterexamples:     1,
		Representation:      []string{"scalar-input-output"},
		SearchPath:          []string{"copy", "increment", "zero"},
		VerificationOutcomes: []string{"forced-composition-heldout"},
		Cost:                ResourceVector{Compute: 5, ExperimentBudget: 2},
	}
	rt := newF0TestRuntime(t)
	rt.MaxCompoundingIterations = 3
	result, err := rt.ImproveAndAcquire(telemetry, spec, hidden, hiddenSpec, hidden)
	if err == nil {
		t.Fatal("expected deterministic recursive compounding rejection under the current bounded acquisition grammar")
	}
	if got := DiagnoseAdaptiveBoundary(telemetry).Class; got != BottleneckSearchSpace {
		t.Fatalf("expected search-space diagnosis for the recursive curriculum, got %s", got)
	}
	if !strings.Contains(err.Error(), "installed acquisition method could not acquire future capability") {
		t.Fatalf("unexpected recursive future-transfer rejection: %v", err)
	}
	if result.Method.ID != "" || result.Future.Capability.ID != "" {
		t.Fatalf("rejected recursive transfer returned a retained capability: method=%q future=%q", result.Method.ID, result.Future.Capability.ID)
	}
}

func mustDecodeProcedure(t *testing.T, artifact string) AcquisitionProcedure {
	t.Helper()
	p, err := decodeAcquisitionProcedure(artifact)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReplicatedCapabilityCompoundingCore(t *testing.T) {
	m, e := RunReplicatedCompoundingV2(12)
	if e != nil {
		t.Fatal(e)
	}
	if m["valid"] != 0 || m["baseline_degenerate"] != 1 || m["wins"] != 0 {
		t.Fatalf("replication guard did not preserve invalidation: %#v", m)
	}
}
