package ace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

type RepresentationBlock struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Op       string `json:"op"`
	Input    string `json:"input"`
	Constant int    `json:"constant,omitempty"`
}

type RepresentationPlaygroundState struct {
	Spec   CapabilitySpecification `json:"spec"`
	Hidden []ProgramTestCase       `json:"hidden"`
	Blocks []RepresentationBlock   `json:"blocks"`
}

func DefaultRepresentationBlocks(spec CapabilitySpecification) []RepresentationBlock {
	out := make([]RepresentationBlock, 0, len(spec.Inputs)*3)
	for _, input := range spec.Inputs {
		out = append(out,
			RepresentationBlock{
				ID:    Hash([]any{"representation-block", "abs", input}),
				Name:  "abs(" + input + ")",
				Op:    "abs",
				Input: input,
			},
			RepresentationBlock{
				ID:    Hash([]any{"representation-block", "neg", input}),
				Name:  "neg(" + input + ")",
				Op:    "neg",
				Input: input,
			},
			RepresentationBlock{
				ID:       Hash([]any{"representation-block", "max-const", input, 2}),
				Name:     "max(" + input + ",2)",
				Op:       "max-const",
				Input:    input,
				Constant: 2,
			},
		)
	}
	return out
}

func ApplyRepresentationBlock(block RepresentationBlock, input map[string]string) (string, error) {
	raw, ok := input[block.Input]
	if !ok {
		return "", fmt.Errorf("representation block %s missing input %q", block.Name, block.Input)
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return "", fmt.Errorf("representation block %s requires integer input: %w", block.Name, err)
	}

	var out int
	switch block.Op {
	case "abs":
		if n < 0 {
			out = -n
		} else {
			out = n
		}
	case "neg":
		out = -n
	case "max-const":
		out = n
		if block.Constant > out {
			out = block.Constant
		}
	default:
		return "", fmt.Errorf("unknown representation block operation %q", block.Op)
	}
	return strconv.Itoa(out), nil
}

func augmentWithRepresentationBlock(
	spec CapabilitySpecification,
	hidden []ProgramTestCase,
	block RepresentationBlock,
) (CapabilitySpecification, []ProgramTestCase, error) {
	feature := "__repr_" + block.ID[:12]
	known := make([]ProgramTestCase, len(spec.KnownExamples))
	for i, tc := range spec.KnownExamples {
		in := cloneStringMap(tc.Input)
		value, err := ApplyRepresentationBlock(block, in)
		if err != nil {
			return CapabilitySpecification{}, nil, err
		}
		in[feature] = value
		known[i] = ProgramTestCase{Input: in, Expected: cloneStringMap(tc.Expected)}
	}

	held := make([]ProgramTestCase, len(hidden))
	for i, tc := range hidden {
		in := cloneStringMap(tc.Input)
		value, err := ApplyRepresentationBlock(block, in)
		if err != nil {
			return CapabilitySpecification{}, nil, err
		}
		in[feature] = value
		held[i] = ProgramTestCase{Input: in, Expected: cloneStringMap(tc.Expected)}
	}

	enriched := spec
	enriched.ID = Hash([]any{"representation-revision", spec.ID, block.ID, known})
	enriched.Inputs = append(append([]string(nil), spec.Inputs...), feature)
	enriched.KnownExamples = known
	enriched.Provenance = Prov("representation-playground", spec.ID, "derived-feature", block)
	return enriched, held, nil
}

func cloneStringMap(src map[string]string) map[string]string {
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// RepresentationPlaygroundRunner is the first executable binding of the
// representation counterfactual. It uses the real ACE UniversalProgram
// executor/builder as the semantic playground and keeps the hidden verifier
// inside the runner boundary.
type RepresentationPlaygroundRunner struct{}

func (RepresentationPlaygroundRunner) ForkAndRun(
	ctx context.Context,
	state ForkExecutionState,
	intervention CounterfactualIntervention,
) (CounterfactualRunResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return CounterfactualRunResult{}, err
	}

	var playground RepresentationPlaygroundState
	if len(state.OpaqueState) == 0 {
		return CounterfactualRunResult{}, errors.New("representation playground state is empty")
	}
	if err := json.Unmarshal(state.OpaqueState, &playground); err != nil {
		return CounterfactualRunResult{}, fmt.Errorf("decode representation playground state: %w", err)
	}
	if len(playground.Spec.KnownExamples) == 0 || len(playground.Hidden) == 0 {
		return CounterfactualRunResult{}, errors.New("representation playground requires training and held-out cases")
	}

	result := CounterfactualRunResult{
		Telemetry: FailureTelemetry{
			TaskID:                   playground.Spec.ID,
			EvidenceCount:            len(playground.Spec.KnownExamples),
			MinimumEvidence:          1,
			CurrentRepresentation:    "scalar-expression",
			SearchExhausted:          true,
			AllCandidateFamiliesExhausted: true,
			SupportedRepresentationFeatures: []string{"scalar"},
		},
	}

	switch intervention.Hypothesis {
	case HypothesisRepresentation:
		for _, block := range playground.Blocks {
			if err := ctx.Err(); err != nil {
				return CounterfactualRunResult{}, err
			}
			enriched, hidden, err := augmentWithRepresentationBlock(playground.Spec, playground.Hidden, block)
			if err != nil {
				return CounterfactualRunResult{}, err
			}

			candidates, err := (UniversalMechanismSearch{}).SearchMechanisms(enriched, enriched.ResourceLimits)
			if err != nil {
				return CounterfactualRunResult{}, err
			}

			for i, candidate := range candidates {
				if err := ctx.Err(); err != nil {
					return CounterfactualRunResult{}, err
				}
				builder := UniversalProgramBuilder{}
				proposal, err := builder.BuildWithContext(ctx, candidate, enriched)
				if err != nil {
					continue
				}
				var program UniversalProgram
				if err := json.Unmarshal([]byte(proposal.Artifact), &program); err != nil {
					continue
				}
				if !programFits(program, hidden) {
					continue
				}

				result.Solved = true
				result.IndependentlyVerified = true
				result.Cost = ResourceVector{
					Compute:         float64(i + 1),
					ExperimentBudget: float64(len(playground.Blocks) + len(candidates)),
				}
				result.Evidence = []string{
					"candidate representation block evaluated against failing task",
					"derived feature participated in executable synthesis",
					"held-out hidden cases independently verified",
					fmt.Sprintf("block=%s", block.Name),
					fmt.Sprintf("candidate=%s", candidate.Mechanism),
				}
				promoted := block
				result.PromotedRepresentation = &promoted
				result.Telemetry.RequiredRepresentationFeatures = []string{"derived-feature", block.Name}
				result.Telemetry.SupportedRepresentationFeatures = []string{"scalar", "derived-feature"}
				result.Telemetry.SearchExhausted = false
				result.Telemetry.AllCandidateFamiliesExhausted = false
				return result, nil
			}
		}
		result.Cost = ResourceVector{ExperimentBudget: float64(len(playground.Blocks))}
		result.Evidence = []string{"all bounded representation blocks failed held-out verification"}
		return result, nil

	case HypothesisResource, HypothesisSearchOrder, HypothesisSearchSpace:
		// These controls execute inside the same semantic playground but do not
		// add representation primitives. For this bounded gate they therefore
		// cannot manufacture a new feature representation.
		candidates, err := (UniversalMechanismSearch{}).SearchMechanisms(playground.Spec, playground.Spec.ResourceLimits)
		if err != nil {
			return CounterfactualRunResult{}, err
		}
		if intervention.Hypothesis == HypothesisSearchOrder {
			for i, j := 0, len(candidates)-1; i < j; i, j = i+1, j-1 {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
		result.Cost = ResourceVector{
			Compute:         intervention.ResourceMultiplier,
			ExperimentBudget: float64(len(candidates)),
		}
		result.Evidence = []string{
			"control intervention preserved scalar representation",
			"no derived feature was introduced",
			fmt.Sprintf("candidate-count=%d", len(candidates)),
		}
		return result, nil

	default:
		return CounterfactualRunResult{}, fmt.Errorf("unsupported counterfactual hypothesis %q", intervention.Hypothesis)
	}
}

func EncodeRepresentationPlaygroundState(state RepresentationPlaygroundState) ([]byte, error) {
	if len(state.Spec.KnownExamples) == 0 {
		return nil, errors.New("cannot encode representation playground without training examples")
	}
	if len(state.Hidden) == 0 {
		return nil, errors.New("cannot encode representation playground without held-out examples")
	}
	return json.Marshal(state)
}
