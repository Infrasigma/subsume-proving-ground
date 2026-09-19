package ace

import (
	"bytes"
	"testing"
	"time"
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
	if err != nil {
		t.Logf("FORCED_COMPOSITION method synthesis: selected_depth=0 candidate_depths=%v generated=%d evaluations=%d verified_by_depth=%v diagnosis=%s error=%q",
			depthCounts, len(methodCandidates), len(evals), verifiedByDepth, methodDiagnosis.Class, err.Error())
	} else {
		selectedDepth := procedureDepthForDiagnostic(t, method)
		t.Logf("FORCED_COMPOSITION method synthesis: selected_depth=%d candidate_depths=%v generated=%d evaluations=%d verified_by_depth=%v diagnosis=%s",
			selectedDepth, depthCounts, len(methodCandidates), len(evals), verifiedByDepth, methodDiagnosis.Class)
	}

	log := &DiagnosticLog{}
	rt := AdaptiveAcquisitionRuntime{
		EnableAbstractionLearning: true,
		Diagnostics: log,
		MaxCompoundingIterations: 3,
		CompoundingTimeout: 30 * time.Second,
	}
	result, runtimeErr := rt.ImproveAndAcquire(
		telemetry, spec, hidden, hiddenSpec, hidden,
	)
	t.Logf("FORCED_COMPOSITION adaptive runtime: err=%v method=%q procedure=%q abstractions=%d history=%d trace=%v",
		runtimeErr, result.Method.ID, result.Method.Procedure,
		len(rt.Abstractions.Abstractions), len(rt.AbstractionHistory), result.Trace)

	if runtimeErr != nil {
		t.Fatalf("T2 recursive runtime failed to defeat depth-3 curriculum: %v", runtimeErr)
	}
	if result.Future.Capability.ID == "" || result.Future.Artifact == "" {
		t.Fatal("T2 recursive runtime did not retain a verified future capability")
	}
	if len(rt.Abstractions.Abstractions) < 1 {
		t.Fatal("T2 recursive runtime did not promote a partial acquisition procedure")
	}
	p, err := decodeAcquisitionProcedure(result.Method.Procedure)
	if err != nil {
		t.Fatal(err)
	}
	if len(abstractionDependencies(p)) == 0 {
		t.Fatalf("T2 final method does not invoke a promoted abstraction: %#v", p)
	}
	t2RecursiveTrace := false
	for _, trace := range result.Trace {
		if trace == "T2-recursive:true" {
			t2RecursiveTrace = true
		}
	}
	if !t2RecursiveTrace {
		t.Fatal("runtime result did not record recursive T2 execution")
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
