package ace

import (
	"bytes"
	"testing"
)

func TestLayerCRejectionTelemetry(t *testing.T) {
	p := AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{{Op: "reverse"}, {Op: "rotate", Arg: 1}}}
	log := &DiagnosticLog{}
	_, err := DiscoverReusableAbstractionWithDiagnostics([]AbstractionObservation{
		{TaskStructure: "A", Procedure: p, Verified: false, HeldOut: true},
		{TaskStructure: "A", Procedure: p, Verified: true, HeldOut: true},
		{TaskStructure: "B", Procedure: p, Verified: true, HeldOut: true},
		{TaskStructure: "C", Procedure: AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{{Op: "reverse"}}}, Verified: true, HeldOut: true},
	}, 2, log)
	if err != nil {
		t.Fatal(err)
	}
	events := log.Events()
	if len(events) < 4 {
		t.Fatalf("expected proposal/rejection/selection telemetry, got %d events", len(events))
	}
	foundUnverified, foundProposed, foundSelected := false, false, false
	for _, e := range events {
		switch e.Predicate {
		case "observation.Verified == true":
			foundUnverified = true
		case "verified + held-out + non-trivial composition":
			foundProposed = foundProposed || e.Accepted
		case "score is maximal among candidates passing evidence gate":
			foundSelected = foundSelected || e.Accepted
		}
		if e.CandidateJSON == "" {
			t.Fatalf("diagnostic event lost raw candidate state: %#v", e)
		}
	}
	if !foundUnverified || !foundProposed || !foundSelected {
		t.Fatalf("missing Layer C telemetry: unverified=%v proposed=%v selected=%v events=%#v", foundUnverified, foundProposed, foundSelected, events)
	}
	var buf bytes.Buffer
	if err := log.WriteJSONL(&buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Fatal("JSONL diagnostic dump is empty")
	}
	t.Logf("Layer C JSONL:\n%s", buf.String())
}

func TestLayerECheatCandidateTelemetry(t *testing.T) {
	spec, tests, err := DeriveCapabilitySpecification(Task{ID: "diagnostic-cheat", Goal: "y=x+1", Budget: ResourceVector{Compute: 100, Memory: 100, Storage: 100, TimeMS: 1000, ExperimentBudget: 100}})
	if err != nil {
		t.Fatal(err)
	}
	log := &DiagnosticLog{}
	cheat := ArchitectureCandidate{
		ID: "diagnostic-cheat", Mechanism: "increment",
		Interfaces: []string{"typed-key-value-input", "executable-program"},
		Advantage: "known-good affine increment control",
		Assumptions: "bounded integer primitive",
		Resources: spec.ResourceLimits,
		Tests: spec.AcceptanceTests,
	}
	a := AutonomousAcquirer{Builder: ProgramBuilder{}, Diagnostics: log}
	_, verification, err := a.EvaluateLayerECandidate(spec, tests, cheat)
	if err != nil || verification.Status != "verified" {
		t.Fatalf("known-good cheat candidate was rejected: err=%v verification=%#v events=%#v", err, verification, log.Events())
	}
	t.Logf("Layer E cheat candidate JSONL:")
	var buf bytes.Buffer
	if err := log.WriteJSONL(&buf); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", buf.String())
}

func TestLayerENaturalCandidateTelemetry(t *testing.T) {
	path := t.TempDir() + "/diagnostic-registry.json"
	reg, err := NewPersistentRegistry(path)
	if err != nil { t.Fatal(err) }
	log := &DiagnosticLog{}
	a := AutonomousAcquirer{Registry: reg, Builder: ProgramBuilder{}, Diagnostics: log}
	_, v, err := a.Acquire(Task{ID: "diagnostic-natural", Goal: "y=x+1", Requirements: []string{"x"}, Budget: ResourceVector{Compute: 100, Memory: 100, Storage: 100, TimeMS: 1000, ExperimentBudget: 10}})
	if err != nil || v.Status != "verified" {
		t.Fatalf("natural Layer E control acquisition failed: err=%v verification=%#v events=%#v", err, v, log.Events())
	}
	var buf bytes.Buffer
	if err := log.WriteJSONL(&buf); err != nil { t.Fatal(err) }
	t.Logf("Layer E natural candidate JSONL:\n%s", buf.String())
}

func TestLayerCAdaptiveRuntimeTelemetry(t *testing.T) {
	failed := thresholdCases(1, []int{-4, 0, 3, 7})
	hidden := thresholdCases(1, []int{-9, -2, 2, 6, 15})
	spec, err := GeneralCapabilitySpecification(Task{ID: "diagnostic-c-failure", Goal: "classify", Requirements: []string{"x"}, Structure: []string{"scalar", "conditional"}, Budget: ResourceVector{Compute: 200, Memory: 100, TimeMS: 5000, ExperimentBudget: 50}}, failed)
	if err != nil { t.Fatal(err) }
	future, err := GeneralCapabilitySpecification(Task{ID: "diagnostic-c-future", Goal: "classify", Requirements: []string{"x"}, Structure: []string{"scalar", "piecewise", "conditional"}, Budget: ResourceVector{Compute: 200, Memory: 100, TimeMS: 5000, ExperimentBudget: 50}}, hidden)
	if err != nil { t.Fatal(err) }
	telemetry := AcquisitionTelemetry{TaskID: "diagnostic-c-failure", TaskStructure: []string{"scalar", "conditional"}, KnownExamples: len(failed), CandidateCount: 1, CandidateFailures: []string{"counterexample"}, Counterexamples: 1, Representation: []string{"scalar-input-output"}, SearchPath: []string{"base"}, VerificationOutcomes: []string{"heldout-failure"}, Cost: ResourceVector{Compute: 2, ExperimentBudget: 2}}
	log := &DiagnosticLog{}
	rt := newF0TestRuntime(t)
	rt.EnableAbstractionLearning = true
	rt.Diagnostics = log
	result, err := rt.ImproveAndAcquire(telemetry, spec, hidden, future, hidden)
	t.Logf("Layer C runtime state: err=%v method_id=%q procedure=%q abstractions=%d abstraction_history=%d", err, result.Method.ID, result.Method.Procedure, len(rt.Abstractions.Abstractions), len(rt.AbstractionHistory))
	var buf bytes.Buffer
	if err := log.WriteJSONL(&buf); err != nil { t.Fatal(err) }
	t.Logf("Layer C adaptive-runtime JSONL:\n%s", buf.String())
}
