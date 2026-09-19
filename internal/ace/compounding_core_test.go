package ace

import (
	"testing"
	"time"
)

func TestRecursiveCapabilityCompoundingCore(t *testing.T) {
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
	rt := AdaptiveAcquisitionRuntime{
		MaxCompoundingIterations: 3,
		CompoundingTimeout:       90 * time.Second,
	}
	result, err := rt.ImproveAndAcquire(telemetry, spec, hidden, hiddenSpec, hidden)
	if err != nil {
		t.Fatal(err)
	}
	if result.Method.ID == "" {
		t.Fatal("recursive T2 run did not install a method")
	}
	if len(abstractionDependencies(mustDecodeProcedure(t, result.Method.Procedure))) == 0 {
		t.Fatalf("recursive T2 method does not invoke an acquired abstraction: %s", result.Method.Procedure)
	}
	if result.Future.Capability.ID == "" || result.Future.Artifact == "" {
		t.Fatal("recursive T2 run did not retain the verified future capability")
	}
	recursive := false
	for _, trace := range result.Trace {
		if trace == "T2-recursive:true" {
			recursive = true
			break
		}
	}
	if !recursive {
		t.Fatalf("recursive T2 run did not record recursive execution: %#v", result.Trace)
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
