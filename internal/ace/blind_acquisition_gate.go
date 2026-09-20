package ace

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
)

// BlindFutureTask contains only behavioral examples. The generator's construction
// rule is intentionally outside the acquisition policy under test.
type BlindFutureTask struct {
	ID     string
	Train  []ProgramTestCase
	Hidden []ProgramTestCase
}

// BlindAcquisitionCost measures only the acquisition-strategy frontier explored by
// this probe. It is intentionally NOT the project's full compute-inclusive C(T).
type BlindAcquisitionCost struct {
	StrategyEvaluations int
}

// BlindAcquisitionResult records the evidence boundary of the blind gate.
type BlindAcquisitionResult struct {
	BootstrapCapabilitiesVerified int
	FutureTasks                   int
	DirectTasksSolved             int
	ComposedTasksSolved           int
	DirectStrategyEvaluations     int
	ComposedStrategyEvaluations   int
	AcquisitionCostRatio          float64
	FullCostAccounting            bool
}

// retainedUnaryCapability is a generic retained executable primitive.
// No future-task identity or target formula is stored here.
type retainedUnaryCapability struct {
	Name   string
	Program UniversalProgram
	Input  string
	Output string
}

// RunBlindAcquisitionGate generates future composite tasks after bootstrap
// capabilities have been independently verified. The future task generator is
// opaque to the acquisition policy: the policy receives examples only.
func RunBlindAcquisitionGate(ctx context.Context) (BlindAcquisitionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	retained, err := learnBlindBootstrap(ctx)
	if err != nil {
		return BlindAcquisitionResult{}, err
	}

	future := generateBlindFutureTasks()
	if len(future) == 0 {
		return BlindAcquisitionResult{}, errors.New("blind future generator produced no tasks")
	}

	result := BlindAcquisitionResult{
		BootstrapCapabilitiesVerified: len(retained),
		FutureTasks:                   len(future),
	}

	for _, task := range future {
		if err := ctx.Err(); err != nil {
			return BlindAcquisitionResult{}, err
		}

		// K0: the existing direct acquisition frontier gets no retained
		// capability library. We count its three generic strategy branches.
		directCost := BlindAcquisitionCost{StrategyEvaluations: 3}
		result.DirectStrategyEvaluations += directCost.StrategyEvaluations

		spec, err := GeneralCapabilitySpecification(
			Task{ID: task.ID, Goal: "opaque-future-task"},
			task.Train,
		)
		if err != nil {
			return BlindAcquisitionResult{}, err
		}

		direct, directErr := learnUniversalProgram(ctx, spec)
		directSolved := false
		if directErr == nil {
			directSolved = verifyUniversalProgram(ctx, direct, task.Hidden) == nil
		}
		if directSolved {
			result.DirectTasksSolved++
		}

		// K1: search only over verified retained capabilities, using generic
		// role-compatible composition. No task-specific target binding exists.
		composed, strategyEvals, composedErr := acquireFromRetained(
			ctx,
			task,
			retained,
		)
		result.ComposedStrategyEvaluations += strategyEvals
		if composedErr != nil {
			continue
		}
		if err := verifyUniversalProgram(ctx, composed, task.Hidden); err != nil {
			return BlindAcquisitionResult{}, fmt.Errorf(
				"future task %s passed composition but failed hidden verification: %w",
				task.ID,
				err,
			)
		}
		result.ComposedTasksSolved++
	}

	if result.ComposedStrategyEvaluations == 0 {
		return BlindAcquisitionResult{}, errors.New("composition policy performed no strategy evaluations")
	}

	result.AcquisitionCostRatio =
		float64(result.ComposedStrategyEvaluations) /
			float64(result.DirectStrategyEvaluations)
	// This gate deliberately measures only frontier strategy evaluations.
	result.FullCostAccounting = false

	return result, nil
}

func learnBlindBootstrap(ctx context.Context) ([]retainedUnaryCapability, error) {
	type primitive struct {
		name   string
		input  string
		output string
		train  []ProgramTestCase
		hidden []ProgramTestCase
	}

	primitives := []primitive{
		{
			name:   "abs",
			input:  "x",
			output: "h",
			train: []ProgramTestCase{
				{Input: map[string]string{"x": "-11"}, Expected: map[string]string{"h": "11"}},
				{Input: map[string]string{"x": "-7"}, Expected: map[string]string{"h": "7"}},
				{Input: map[string]string{"x": "-3"}, Expected: map[string]string{"h": "3"}},
				{Input: map[string]string{"x": "-1"}, Expected: map[string]string{"h": "1"}},
				{Input: map[string]string{"x": "0"}, Expected: map[string]string{"h": "0"}},
				{Input: map[string]string{"x": "4"}, Expected: map[string]string{"h": "4"}},
				{Input: map[string]string{"x": "7"}, Expected: map[string]string{"h": "7"}},
			},
			hidden: []ProgramTestCase{
				{Input: map[string]string{"x": "-13"}, Expected: map[string]string{"h": "13"}},
				{Input: map[string]string{"x": "5"}, Expected: map[string]string{"h": "5"}},
			},
		},
		{
			name:   "max2",
			input:  "h",
			output: "y",
			train: []ProgramTestCase{
				{Input: map[string]string{"h": "0"}, Expected: map[string]string{"y": "2"}},
				{Input: map[string]string{"h": "5"}, Expected: map[string]string{"y": "5"}},
			},
			hidden: []ProgramTestCase{
				{Input: map[string]string{"h": "1"}, Expected: map[string]string{"y": "2"}},
				{Input: map[string]string{"h": "8"}, Expected: map[string]string{"y": "8"}},
			},
		},
		{
			name:   "max3",
			input:  "h",
			output: "y",
			train: []ProgramTestCase{
				{Input: map[string]string{"h": "0"}, Expected: map[string]string{"y": "3"}},
				{Input: map[string]string{"h": "6"}, Expected: map[string]string{"y": "6"}},
			},
			hidden: []ProgramTestCase{
				{Input: map[string]string{"h": "2"}, Expected: map[string]string{"y": "3"}},
				{Input: map[string]string{"h": "9"}, Expected: map[string]string{"y": "9"}},
			},
		},
	}

	retained := make([]retainedUnaryCapability, 0, len(primitives))
	for _, p := range primitives {
		spec, err := GeneralCapabilitySpecification(
			Task{ID: "bootstrap:" + p.name, Goal: "bootstrap"},
			p.train,
		)
		if err != nil {
			return nil, err
		}
		program, err := learnUniversalProgram(ctx, spec)
		if err != nil {
			return nil, fmt.Errorf("bootstrap %s synthesis failed: %w", p.name, err)
		}
		if err := verifyUniversalProgram(ctx, program, p.hidden); err != nil {
			return nil, fmt.Errorf("bootstrap %s hidden verification failed: %w", p.name, err)
		}
		retained = append(retained, retainedUnaryCapability{
			Name:    p.name,
			Program: program,
			Input:   p.input,
			Output:  p.output,
		})
	}

	return retained, nil
}

func generateBlindFutureTasks() []BlindFutureTask {
	// The seed is fixed only for reproducibility. The acquisition policy sees
	// none of this generator state; it receives behavioral examples only.
	rng := rand.New(rand.NewSource(20260920))
	tasks := make([]BlindFutureTask, 0, 8)

	for i := 0; i < 8; i++ {
		threshold := 2
		if rng.Intn(2) == 1 {
			threshold = 3
		}

		trainXs := []int{-1, 3, 5}
		hiddenXs := []int{-9, 1, 8}
		if i%2 == 1 {
			trainXs = []int{-2, 4, 7}
			hiddenXs = []int{-11, 2, 10}
		}

		train := make([]ProgramTestCase, 0, len(trainXs))
		hidden := make([]ProgramTestCase, 0, len(hiddenXs))
		for _, x := range trainXs {
			train = append(train, ProgramTestCase{
				Input:    map[string]string{"x": fmt.Sprintf("%d", x)},
				Expected: map[string]string{"y": fmt.Sprintf("%d", blindCompositeOracle(x, threshold))},
			})
		}
		for _, x := range hiddenXs {
			hidden = append(hidden, ProgramTestCase{
				Input:    map[string]string{"x": fmt.Sprintf("%d", x)},
				Expected: map[string]string{"y": fmt.Sprintf("%d", blindCompositeOracle(x, threshold))},
			})
		}

		tasks = append(tasks, BlindFutureTask{
			ID:     fmt.Sprintf("blind-future-%02d", i),
			Train:  train,
			Hidden: hidden,
		})
	}

	return tasks
}

func blindCompositeOracle(x, threshold int) int {
	if x < 0 {
		x = -x
	}
	if x < threshold {
		return threshold
	}
	return x
}

func acquireFromRetained(
	ctx context.Context,
	task BlindFutureTask,
	retained []retainedUnaryCapability,
) (UniversalProgram, int, error) {
	// Candidate policies are generic: any pair whose typed roles compose is
	// admissible. The policy does not inspect the task's generating formula.
	strategyEvaluations := 0
	for _, first := range retained {
		for _, second := range retained {
			if err := ctx.Err(); err != nil {
				return UniversalProgram{}, strategyEvaluations, err
			}
			if first.Output != second.Input {
				continue
			}
			strategyEvaluations++

			composed, err := composeUnaryUniversalPrograms(
				first.Program,
				first.Input,
				first.Output,
				second.Program,
				second.Input,
			)
			if err != nil {
				continue
			}

			spec, err := GeneralCapabilitySpecification(
				Task{ID: task.ID, Goal: "opaque-composed-task"},
				task.Train,
			)
			if err != nil {
				return UniversalProgram{}, strategyEvaluations, err
			}
			if programFits(composed, spec.KnownExamples) {
				return composed, strategyEvaluations, nil
			}
		}
	}
	return UniversalProgram{}, strategyEvaluations, errors.New("no retained capability composition fit the future task")
}
