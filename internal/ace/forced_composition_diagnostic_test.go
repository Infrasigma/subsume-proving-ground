package ace

import (
	"bytes"
	"strings"
	"testing"
)

func procedureDepthForDiagnostic(t *testing.T, artifact AcquisitionMethodArtifact) int {
	t.Helper()
	p, err := decodeAcquisitionProcedure(artifact.Procedure)
	if err != nil {
		t.Fatalf("invalid synthesized method procedure: %v", err)
	}
	return len(p.Steps)
}

func TestForcedCompositionAdaptiveRuntimeTelemetry(t *testing.T) {
	task, train, _, err := DeepCompositionFamily{}.Generate(1, false)
	if err != nil {
		t.Fatal(err)
	}
	hiddenTask, hidden, _, err := DeepCompositionFamily{}.Generate(1, true)
	if err != nil {
		t.Fatal(err)
	}

	// Prove the curriculum is not solvable by the legacy primitive builder.
	primitiveCandidates := []ArchitectureCandidate{
		{ID: "copy", Mechanism: "copy", Interfaces: []string{"ExecutableProgram"}},
		{ID: "increment", Mechanism: "increment", Interfaces: []string{"ExecutableProgram"}},
		{ID: "zero", Mechanism: "zero", Interfaces: []string{"ExecutableProgram"}},
	}
	spec, err := GeneralCapabilitySpecification(task, train)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range primitiveCandidates {
		p, buildErr := (ProgramBuilder{}).Build(c, spec)
		if buildErr == nil && programFitsJSON(p.Artifact, train) {
			t.Fatalf("forced-composition curriculum was unexpectedly solved by primitive %q", c.Mechanism)
		}
	}
	hiddenSpec, err := GeneralCapabilitySpecification(hiddenTask, hidden)
	if err != nil {
		t.Fatal(err)
	}

	telemetry := AcquisitionTelemetry{
		TaskID: "forced-composition",
		TaskStructure: []string{"scalar", "composition", "depth-3"},
		KnownExamples: len(train),
		CandidateCount: 3,
		CandidateFailures: []string{"single-primitive curriculum control"},
		Counterexamples: 1,
		Representation: []string{"scalar-input-output"},
		SearchPath: []string{"copy", "increment", "zero"},
		VerificationOutcomes: []string{"forced-composition-heldout"},
		Cost: ResourceVector{Compute: 5, ExperimentBudget: 2},
	}
	diagnosis := DiagnoseBottleneck(telemetry)

	methodCandidates := AutonomousMethodCandidates(diagnosis, spec, spec.ResourceLimits)
	depthCounts := map[int]int{}
	for _, c := range methodCandidates {
		p, err := decodeAcquisitionProcedure(c.Artifact.Procedure)
		if err != nil {
			t.Fatal(err)
		}
		depthCounts[len(p.Steps)]++
	}

	method, methodDiagnosis, evals, err := AutonomousMethodImprovement(
		telemetry, spec, hidden, nil, nil,
	)
	verifiedByDepth := map[int]int{}
	for _, e := range evals {
		if !e.Verified {
			continue
		}
		p, decodeErr := decodeAcquisitionProcedure(e.Candidate.Procedure)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		verifiedByDepth[len(p.Steps)]++
	}
	if err == nil {
		selectedDepth := procedureDepthForDiagnostic(t, method)
		t.Fatalf("expected bounded method synthesis rejection, but selected depth=%d diagnosis=%s", selectedDepth, methodDiagnosis.Class)
	}
	t.Logf("FORCED_COMPOSITION method synthesis: selected_depth=0 candidate_depths=%v generated=%d evaluations=%d verified_by_depth=%v diagnosis=%s error=%q",
		depthCounts, len(methodCandidates), len(evals), verifiedByDepth, methodDiagnosis.Class, err.Error())
	if methodDiagnosis.Class != BottleneckSearchSpace {
		t.Fatalf("expected search-space diagnosis, got %s", methodDiagnosis.Class)
	}
	if len(evals) != len(methodCandidates) {
		t.Fatalf("expected every generated candidate to be evaluated, generated=%d evaluations=%d", len(methodCandidates), len(evals))
	}

	log := &DiagnosticLog{}
	rt := newF0TestRuntime(t)
	rt.EnableAbstractionLearning = true
	rt.Diagnostics = log
	rt.MaxCompoundingIterations = 3
	result, runtimeErr := rt.ImproveAndAcquire(
		telemetry, spec, hidden, hiddenSpec, hidden,
	)
	t.Logf("FORCED_COMPOSITION adaptive runtime: err=%v method=%q procedure=%q abstractions=%d history=%d trace=%v",
		runtimeErr, result.Method.ID, result.Method.Procedure,
		len(rt.Abstractions.Abstractions), len(rt.AbstractionHistory), result.Trace)

	if runtimeErr == nil {
		t.Fatal("expected T2 recursive runtime to reject the unexpanded future frontier")
	}
	if !strings.Contains(runtimeErr.Error(), "installed acquisition method could not acquire future capability") {
		t.Fatalf("unexpected T2 recursive rejection: %v", runtimeErr)
	}
	if result.Method.ID != "" || result.Future.Capability.ID != "" {
		t.Fatalf("rejected T2 transfer returned a retained capability: method=%q future=%q", result.Method.ID, result.Future.Capability.ID)
	}
	if len(rt.Abstractions.Abstractions) < 1 {
		t.Fatal("T2 recursive runtime did not preserve the promoted partial acquisition abstraction")
	}

	for _, e := range log.Events() {
		t.Logf("FORCED_COMPOSITION telemetry: stage=%s predicate=%s accepted=%v detail=%s candidate=%s",
			e.Stage, e.Predicate, e.Accepted, e.Detail, e.CandidateID)
	}
	var jsonl bytes.Buffer
	if err := log.WriteJSONL(&jsonl); err != nil {
		t.Fatal(err)
	}
	t.Logf("FORCED_COMPOSITION T2 JSONL:\n%s", jsonl.String())
}
