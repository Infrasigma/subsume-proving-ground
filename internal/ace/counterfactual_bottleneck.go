package ace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type CounterfactualHypothesis string

const (
	HypothesisResource        CounterfactualHypothesis = "resource"
	HypothesisSearchOrder     CounterfactualHypothesis = "search-order"
	HypothesisSearchSpace     CounterfactualHypothesis = "search-space"
	HypothesisRepresentation  CounterfactualHypothesis = "representation"
)

type CounterfactualIntervention struct {
	Hypothesis              CounterfactualHypothesis
	ResourceMultiplier      float64
	SearchOrder             string
	ExpandGeneratorSpace    bool
	AllowRepresentationRev  bool
}

// ForkExecutionState is an immutable, serializable branch point. The actual
// executor owns the opaque payload; the diagnosis layer never interprets it.
type ForkExecutionState struct {
	ID          string
	ParentTask  string
	Hypothesis  CounterfactualHypothesis
	Telemetry   FailureTelemetry
	OpaqueState []byte
}

type CounterfactualRunResult struct {
	Solved               bool
	IndependentlyVerified bool
	Cost                 ResourceVector
	Evidence             []string
	Telemetry            FailureTelemetry
}

type CounterfactualRunner interface {
	ForkAndRun(context.Context, ForkExecutionState, CounterfactualIntervention) (CounterfactualRunResult, error)
}

type CounterfactualTrial struct {
	Hypothesis   CounterfactualHypothesis
	Intervention CounterfactualIntervention
	ForkID       string
	Result       CounterfactualRunResult
}

type CounterfactualDiagnosis struct {
	Diagnosis BottleneckDiagnosis
	Trials    []CounterfactualTrial
	Discriminated bool
}

// BuildCounterfactualInterventions creates single-factor interventions.
// A representation hypothesis permits representation revision but does not
// prescribe the representation. Generator expansion similarly grants a
// discovery opportunity without naming the answer.
func BuildCounterfactualInterventions() []CounterfactualIntervention {
	return []CounterfactualIntervention{
		{
			Hypothesis:         HypothesisResource,
			ResourceMultiplier: 2,
		},
		{
			Hypothesis:     HypothesisSearchOrder,
			SearchOrder:    "alternate",
		},
		{
			Hypothesis:           HypothesisSearchSpace,
			ExpandGeneratorSpace: true,
		},
		{
			Hypothesis:             HypothesisRepresentation,
			AllowRepresentationRev: true,
		},
	}
}

func ForkFailureExecution(t FailureTelemetry, opaqueState []byte, hypothesis CounterfactualHypothesis) (ForkExecutionState, error) {
	if t.TaskID == "" {
		return ForkExecutionState{}, errors.New("cannot fork failure execution without task identity")
	}
	cloned := append([]byte(nil), opaqueState...)
	id := Hash([]any{
		"counterfactual-fork",
		t,
		hypothesis,
		cloned,
	})
	return ForkExecutionState{
		ID:          id,
		ParentTask:  t.TaskID,
		Hypothesis:  hypothesis,
		Telemetry:   t,
		OpaqueState: cloned,
	}, nil
}

func RunDiscriminatingBottleneckExperiments(
	ctx context.Context,
	failure FailureTelemetry,
	opaqueState []byte,
	runner CounterfactualRunner,
) (CounterfactualDiagnosis, error) {
	if runner == nil {
		return CounterfactualDiagnosis{}, errors.New("counterfactual runner unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	interventions := BuildCounterfactualInterventions()
	trials := make([]CounterfactualTrial, 0, len(interventions))

	for _, intervention := range interventions {
		if err := ctx.Err(); err != nil {
			return CounterfactualDiagnosis{}, err
		}

		fork, err := ForkFailureExecution(failure, opaqueState, intervention.Hypothesis)
		if err != nil {
			return CounterfactualDiagnosis{}, err
		}

		result, err := runner.ForkAndRun(ctx, fork, intervention)
		if err != nil {
			return CounterfactualDiagnosis{}, fmt.Errorf(
				"counterfactual %s failed to execute: %w",
				intervention.Hypothesis,
				err,
			)
		}

		trials = append(trials, CounterfactualTrial{
			Hypothesis:   intervention.Hypothesis,
			Intervention: intervention,
			ForkID:       fork.ID,
			Result:       result,
		})
	}

	diagnosis, discriminated := selectDiscriminatingDiagnosis(failure, trials)
	return CounterfactualDiagnosis{
		Diagnosis:     diagnosis,
		Trials:        trials,
		Discriminated: discriminated,
	}, nil
}

func selectDiscriminatingDiagnosis(
	failure FailureTelemetry,
	trials []CounterfactualTrial,
) (BottleneckDiagnosis, bool) {
	var winners []CounterfactualTrial
	for _, trial := range trials {
		if !trial.Result.Solved || !trial.Result.IndependentlyVerified {
			continue
		}
		winners = append(winners, trial)
	}

	if len(winners) != 1 {
		return BottleneckDiagnosis{
			Class:      BottleneckUnknown,
			Reason:     "counterfactual interventions did not isolate a unique independently verified causal explanation",
			Confidence: 0.0,
			Evidence:   []string{fmt.Sprintf("verified-winners=%d", len(winners))},
		}, false
	}

	w := winners[0]
	var class BottleneckClass
	switch w.Hypothesis {
	case HypothesisResource:
		class = BottleneckResource
	case HypothesisSearchOrder:
		class = BottleneckSearchOrder
	case HypothesisSearchSpace:
		class = BottleneckSearchSpace
	case HypothesisRepresentation:
		class = BottleneckRepresentation
	default:
		return BottleneckDiagnosis{
			Class:    BottleneckUnknown,
			Reason:   "unknown counterfactual hypothesis cannot be promoted",
			Evidence: []string{string(w.Hypothesis)},
		}, false
	}

	evidence := append([]string(nil), w.Result.Evidence...)
	evidence = append(evidence,
		fmt.Sprintf("fork=%s", w.ForkID),
		"winner independently verified",
	)
	return BottleneckDiagnosis{
		Class:      class,
		Reason:     "unique counterfactual intervention produced an independently verified improvement",
		Confidence: 0.99,
		Evidence:   evidence,
	}, true
}

func EncodeCounterfactualDiagnosis(d CounterfactualDiagnosis) ([]byte, error) {
	return json.Marshal(d)
}
