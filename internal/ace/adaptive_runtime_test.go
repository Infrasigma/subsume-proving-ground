package ace

import "testing"

func TestAdaptiveAcquisitionRuntimeCausalCompounding(t *testing.T) {
	failedCases := thresholdCases(5, []int{-3, 0, 5, 7})
	methodHidden := thresholdCases(5, []int{-9, 1, 6, 12})
	failedSpec, err := GeneralCapabilitySpecification(Task{ID: "opaque-failure", Goal: "classify input", Requirements: []string{"x"}, Structure: []string{"scalar", "conditional"}, Budget: ResourceVector{Compute: 100, Memory: 100, TimeMS: 5000, ExperimentBudget: 20}}, failedCases)
	if err != nil { t.Fatal(err) }
	futureCases := []ProgramTestCase{
		methodInputOutputExample(-6, -5), methodInputOutputExample(-1, 0),
		methodInputOutputExample(0, 0), methodInputOutputExample(3, 6),
		methodInputOutputExample(8, 16),
	}
	futureSpec, err := GeneralCapabilitySpecification(Task{ID: "opaque-future", Goal: "piecewise transform", Requirements: []string{"x"}, Structure: []string{"scalar", "piecewise", "branch-plus-arithmetic"}, Budget: ResourceVector{Compute: 100, Memory: 100, TimeMS: 5000, ExperimentBudget: 20}}, futureCases)
	if err != nil { t.Fatal(err) }
	telemetry := AcquisitionTelemetry{
		TaskID: "opaque-failure", TaskStructure: []string{"scalar", "conditional"},
		KnownExamples: len(failedCases), CandidateCount: 1,
		CandidateFailures: []string{"independent counterexample mismatch"}, Counterexamples: 1,
		Representation: []string{"scalar-input-output"}, SearchPath: []string{"parameterized-add", "parameterized-mul"},
		VerificationOutcomes: []string{"failed-independent-boundary"}, Cost: ResourceVector{Compute: 1, ExperimentBudget: 1},
	}
	rt := AdaptiveAcquisitionRuntime{}
	result, err := rt.ImproveAndAcquire(telemetry, failedSpec, methodHidden, futureSpec, futureCases)
	if err != nil { t.Fatal(err) }
	if result.Diagnosis.Class != BottleneckSearchSpace { t.Fatalf("unexpected diagnosis: %s", result.Diagnosis.Class) }
	if result.Method.Name == "" || result.Method.Artifact == "" { t.Fatal("method was not materialized as an artifact") }
	if result.Future.Capability.ID == "" || result.Future.Artifact == "" { t.Fatal("future capability was not retained") }
	if len(result.Evaluations) < 2 { t.Fatalf("expected competing method evaluations, got %d", len(result.Evaluations)) }
	if len(result.Trace) < 2 { t.Fatalf("missing installed-method execution trace: %#v", result.Trace) }
}
