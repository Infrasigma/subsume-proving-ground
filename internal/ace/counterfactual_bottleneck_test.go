package ace

import (
	"context"
	"testing"
)

type counterfactualRunnerFunc func(context.Context, ForkExecutionState, CounterfactualIntervention) (CounterfactualRunResult, error)

func (f counterfactualRunnerFunc) ForkAndRun(
	ctx context.Context,
	state ForkExecutionState,
	intervention CounterfactualIntervention,
) (CounterfactualRunResult, error) {
	return f(ctx, state, intervention)
}

func TestCounterfactualExperimentsIsolateRepresentationFailure(t *testing.T) {
	failure := FailureTelemetry{
		TaskID:                         "alien-representation",
		EvidenceCount:                  4,
		MinimumEvidence:                1,
		SearchExhausted:                true,
		AllCandidateFamiliesExhausted: true,
		CurrentRepresentation:          "scalar-expression",
		SupportedRepresentationFeatures: []string{"scalar"},
		RequiredRepresentationFeatures:  []string{"relational-state"},
	}

	runner := counterfactualRunnerFunc(func(
		_ context.Context,
		state ForkExecutionState,
		intervention CounterfactualIntervention,
	) (CounterfactualRunResult, error) {
		result := CounterfactualRunResult{
			Telemetry: state.Telemetry,
			Evidence:  []string{"matched-resource-and-search controls remained failed"},
		}
		if intervention.AllowRepresentationRev {
			result.Solved = true
			result.IndependentlyVerified = true
			result.Evidence = append(result.Evidence, "new representation enabled hidden-task solution")
		}
		return result, nil
	})

	out, err := RunDiscriminatingBottleneckExperiments(
		context.Background(),
		failure,
		[]byte("opaque execution snapshot"),
		runner,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Discriminated {
		t.Fatal("expected unique discriminating diagnosis")
	}
	if out.Diagnosis.Class != BottleneckRepresentation {
		t.Fatalf("got %q want %q", out.Diagnosis.Class, BottleneckRepresentation)
	}
	if len(out.Trials) != 4 {
		t.Fatalf("got %d trials want 4", len(out.Trials))
	}
}

func TestCounterfactualExperimentsRefuseAmbiguousWinner(t *testing.T) {
	failure := FailureTelemetry{
		TaskID:        "ambiguous",
		EvidenceCount: 4,
		MinimumEvidence: 1,
	}

	runner := counterfactualRunnerFunc(func(
		_ context.Context,
		_ ForkExecutionState,
		intervention CounterfactualIntervention,
	) (CounterfactualRunResult, error) {
		return CounterfactualRunResult{
			Solved: true,
			IndependentlyVerified: true,
			Evidence: []string{string(intervention.Hypothesis)},
		}, nil
	})

	out, err := RunDiscriminatingBottleneckExperiments(
		context.Background(),
		failure,
		nil,
		runner,
	)
	if err != nil {
		t.Fatal(err)
	}
	if out.Discriminated {
		t.Fatal("ambiguous interventions were incorrectly promoted")
	}
	if out.Diagnosis.Class != BottleneckUnknown {
		t.Fatalf("got %q want %q", out.Diagnosis.Class, BottleneckUnknown)
	}
}

func TestForkFailureExecutionDoesNotAliasOpaqueState(t *testing.T) {
	failure := FailureTelemetry{TaskID: "fork"}
	original := []byte("snapshot")
	fork, err := ForkFailureExecution(failure, original, HypothesisSearchSpace)
	if err != nil {
		t.Fatal(err)
	}
	original[0] = 'X'
	if string(fork.OpaqueState) != "snapshot" {
		t.Fatalf("fork aliased opaque state: %q", fork.OpaqueState)
	}
}

func TestCounterfactualDiagnosisSerializes(t *testing.T) {
	d := CounterfactualDiagnosis{
		Diagnosis: BottleneckDiagnosis{
			Class:      BottleneckSearchSpace,
			Reason:     "unique generator expansion intervention succeeded",
			Confidence: 0.99,
		},
		Discriminated: true,
	}
	if _, err := EncodeCounterfactualDiagnosis(d); err != nil {
		t.Fatal(err)
	}
}
