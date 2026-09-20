package ace

import (
	"strings"
	"testing"
)

func TestAdaptiveAcquisitionRuntimeCausalCompounding(t *testing.T) {
	failedCases := thresholdCases(2, []int{-3, 0, 2, 7})
	methodHidden := thresholdCases(2, []int{-9, 1, 3, 12})
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
		TaskID: "opaque-failure", TaskStructure: []string{"scalar", "conditional"}, KnownExamples: len(failedCases), CandidateCount: 1,
		CandidateFailures: []string{"candidate family exhausted"}, Counterexamples: 1,
		Representation: []string{"scalar-input-output"}, SearchPath: []string{"parameterized-add", "parameterized-mul"},
		Cost: ResourceVector{Compute: 1, ExperimentBudget: 1},
	}
	rt := newF0TestRuntime(t)
	result, err := rt.ImproveAndAcquire(telemetry, failedSpec, methodHidden, futureSpec, futureCases)
	if err != nil { t.Fatal(err) }
	if result.Diagnosis.Class != BottleneckSearchSpace { t.Fatalf("unexpected diagnosis: %s", result.Diagnosis.Class) }
	if result.Method.Name == "" || result.Method.Artifact == "" { t.Fatal("method was not materialized as an artifact") }
	if result.Future.Capability.ID == "" || result.Future.Artifact == "" { t.Fatal("future capability was not retained") }
	if len(result.Evaluations) < 1 { t.Fatalf("expected at least one recorded T2 verification attempt, got %d", len(result.Evaluations)) }
	if len(result.Trace) < 2 { t.Fatalf("missing installed-method execution trace: %#v", result.Trace) }
}

func TestAdaptiveAcquisitionRuntimeEndogenousAbstractionLearning(t *testing.T) {
	failed := thresholdCases(1, []int{-4, 0, 3, 7})
	hidden := thresholdCases(1, []int{-9, -2, 2, 6, 15})
	spec, err := GeneralCapabilitySpecification(Task{ID: "learn-failure", Goal: "classify", Requirements: []string{"x"}, Structure: []string{"scalar", "conditional"}, Budget: ResourceVector{Compute: 200, Memory: 100, TimeMS: 5000, ExperimentBudget: 50}}, failed)
	if err != nil { t.Fatal(err) }
	future, err := GeneralCapabilitySpecification(Task{ID: "learn-future", Goal: "classify", Requirements: []string{"x"}, Structure: []string{"scalar", "piecewise", "conditional"}, Budget: ResourceVector{Compute: 200, Memory: 100, TimeMS: 5000, ExperimentBudget: 50}}, hidden)
	if err != nil { t.Fatal(err) }
	telemetry := AcquisitionTelemetry{TaskID: "learn-failure", TaskStructure: []string{"scalar", "conditional"}, KnownExamples: len(failed), CandidateCount: 1, CandidateFailures: []string{"counterexample"}, Counterexamples: 1, Representation: []string{"scalar-input-output"}, SearchPath: []string{"base"}, VerificationOutcomes: []string{"heldout-failure"}, Cost: ResourceVector{Compute: 2, ExperimentBudget: 2}}
	rt := newF0TestRuntime(t)
	rt.EnableAbstractionLearning = true
	result, err := rt.ImproveAndAcquire(telemetry, spec, hidden, future, hidden)
	if err == nil {
		t.Fatal("expected deterministic recursive compounding rejection after the verified abstraction failed to expand the future acquisition frontier")
	}
	if got := DiagnoseAdaptiveBoundary(telemetry).Class; got != BottleneckSearchSpace {
		t.Fatalf("expected search-space diagnosis for the observed failure topology, got %s", got)
	}
	if !strings.Contains(err.Error(), "recursive synthesis exhausted promoted frontier without verification") {
		t.Fatalf("unexpected recursive rejection: %v", err)
	}
	if len(rt.Abstractions.Abstractions) != 1 {
		t.Fatalf("expected the partial abstraction to be admitted before transfer rejection, got %d", len(rt.Abstractions.Abstractions))
	}
	if result.Method.ID != "" || result.Future.Capability.ID != "" {
		t.Fatalf("rejected transfer returned a retained capability: method=%q future=%q", result.Method.ID, result.Future.Capability.ID)
	}
}

func TestAcquiredProcedureSurvivesRestart(t *testing.T) { path:=t.TempDir()+"/methods.json";m:=AcquisitionMethodArtifact{ID:"restart-method",Name:"acquired-procedure:test",Artifact:`{"version":1,"steps":[{"op":"reverse"}]}`,Procedure:`{"version":1,"steps":[{"op":"reverse"}]}`};store,err:=NewPersistentMethodRegistry(path);if err!=nil{t.Fatal(err)};if err=store.Install(m);err!=nil{t.Fatal(err)};reloaded,err:=NewPersistentMethodRegistry(path);if err!=nil{t.Fatal(err)};dst:=InstalledMethodRegistry{};if err=reloaded.Restore(&dst);err!=nil{t.Fatal(err)};if len(dst.Methods)!=1||dst.Methods[0].ID!=m.ID{t.Fatalf("restart lost acquired procedure: %#v",dst.Methods)};cs,err:=dst.Apply(CapabilitySpecification{ID:"restart",DesiredBehaviour:"x",Inputs:[]string{"x"},Outputs:[]string{"y"},ResourceLimits:ResourceVector{Compute:10,ExperimentBudget:10}});if err!=nil{t.Fatal(err)};if len(cs)!=3||cs[0].Mechanism!="universal:compositional"{t.Fatalf("restored procedure not executable after restart: %#v",cs)}}
