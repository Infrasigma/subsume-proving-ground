package ace

import "testing"

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

func TestAdaptiveAcquisitionRuntimeEndogenousAbstractionLearning(t *testing.T) {
	failed := thresholdCases(1, []int{-4, 0, 3, 7})
	hidden := thresholdCases(1, []int{-9, -2, 2, 6, 15})
	spec, err := GeneralCapabilitySpecification(Task{ID: "learn-failure", Goal: "classify", Requirements: []string{"x"}, Structure: []string{"scalar", "conditional"}, Budget: ResourceVector{Compute: 200, Memory: 100, TimeMS: 5000, ExperimentBudget: 50}}, failed)
	if err != nil { t.Fatal(err) }
	future, err := GeneralCapabilitySpecification(Task{ID: "learn-future", Goal: "classify", Requirements: []string{"x"}, Structure: []string{"scalar", "piecewise", "conditional"}, Budget: ResourceVector{Compute: 200, Memory: 100, TimeMS: 5000, ExperimentBudget: 50}}, hidden)
	if err != nil { t.Fatal(err) }
	telemetry := AcquisitionTelemetry{TaskID: "learn-failure", TaskStructure: []string{"scalar", "conditional"}, KnownExamples: len(failed), CandidateCount: 1, CandidateFailures: []string{"counterexample"}, Counterexamples: 1, Representation: []string{"scalar-input-output"}, SearchPath: []string{"base"}, VerificationOutcomes: []string{"heldout-failure"}, Cost: ResourceVector{Compute: 2, ExperimentBudget: 2}}
	rt := AdaptiveAcquisitionRuntime{EnableAbstractionLearning: true}
	result, err := rt.ImproveAndAcquire(telemetry, spec, hidden, future, hidden)
	if err != nil { t.Fatal(err) }
	if len(rt.Abstractions.Abstractions) != 1 { t.Fatalf("expected runtime to install one abstraction from verified experience, got %d", len(rt.Abstractions.Abstractions)) }
	a := rt.Abstractions.Abstractions[0]
	if !a.Verification.Independent || a.Verification.Status != "verified" { t.Fatalf("runtime-installed abstraction is not independently verified: %#v", a.Verification) }
	if len(a.Evidence) < 2 { t.Fatalf("runtime abstraction lacks cross-task evidence: %#v", a.Evidence) }
	if result.Future.Capability.ID == "" { t.Fatal("future capability was not retained") }

	before := rt.Abstractions.IDs()[0]
	secondTelemetry := telemetry
	secondTelemetry.TaskID = "learn-failure-2"
	secondTelemetry.TaskStructure = []string{"relational", "conditional"}
	second, err := rt.ImproveAndAcquire(secondTelemetry, spec, hidden, future, hidden)
	if err != nil { t.Fatal(err) }
	if len(rt.Abstractions.Abstractions) < 1 { t.Fatal("endogenous abstraction library regressed after second acquisition") }
	if rt.Abstractions.IDs()[0] != before { t.Fatal("existing acquired abstraction changed identity across later learning") }
	if second.Future.Capability.ID == "" { t.Fatal("second future capability was not retained") }
}

func TestAcquiredProcedureSurvivesRestart(t *testing.T) { path:=t.TempDir()+"/methods.json";m:=AcquisitionMethodArtifact{ID:"restart-method",Name:"acquired-procedure:test",Artifact:`{"version":1,"steps":[{"op":"reverse"}]}`,Procedure:`{"version":1,"steps":[{"op":"reverse"}]}`};store,err:=NewPersistentMethodRegistry(path);if err!=nil{t.Fatal(err)};if err=store.Install(m);err!=nil{t.Fatal(err)};reloaded,err:=NewPersistentMethodRegistry(path);if err!=nil{t.Fatal(err)};dst:=InstalledMethodRegistry{};if err=reloaded.Restore(&dst);err!=nil{t.Fatal(err)};if len(dst.Methods)!=1||dst.Methods[0].ID!=m.ID{t.Fatalf("restart lost acquired procedure: %#v",dst.Methods)};cs,err:=dst.Apply(CapabilitySpecification{ID:"restart",DesiredBehaviour:"x",Inputs:[]string{"x"},Outputs:[]string{"y"},ResourceLimits:ResourceVector{Compute:10,ExperimentBudget:10}});if err!=nil{t.Fatal(err)};if len(cs)!=3||cs[0].Mechanism!="universal:compositional"{t.Fatalf("restored procedure not executable after restart: %#v",cs)}}
