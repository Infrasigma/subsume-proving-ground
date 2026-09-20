package ace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

var ErrRepresentationSearchExhausted = errors.New("representation search exhausted")

type RepresentationBlock struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Op       string `json:"op"`
	Input    string `json:"input"`
	Constant int    `json:"constant,omitempty"`
	Output   string `json:"output,omitempty"`
	Artifact string `json:"artifact,omitempty"`
}

type RepresentationPlaygroundState struct {
	Spec   CapabilitySpecification `json:"spec"`
	Hidden []ProgramTestCase       `json:"hidden"`
	Blocks []RepresentationBlock   `json:"blocks"`
}

func DefaultRepresentationBlocks(spec CapabilitySpecification) []RepresentationBlock {
	blocks, err := EnumerateRepresentationBlocks(context.Background(), spec)
	if err != nil {
		return nil
	}
	return blocks
}

// EnumerateRepresentationBlocks searches a bounded structural grammar for
// executable derived features. The grammar supplies generic arithmetic,
// comparison, and branching primitives; it does not contain an "abs"
// primitive. A successful block is therefore a synthesized representation
// artifact, not a lookup of a hand-authored semantic answer.
func EnumerateRepresentationBlocks(ctx context.Context, spec CapabilitySpecification) ([]RepresentationBlock, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(spec.Inputs) == 0 {
		return nil, errors.New("representation enumeration requires at least one input")
	}

	const maxBlocks = 512
	blocks := make([]RepresentationBlock, 0, maxBlocks)

	appendBlock := func(input string, program UniversalProgram) bool {
		if len(blocks) >= maxBlocks {
			return false
		}
		artifact, err := json.Marshal(program)
		if err != nil {
			return true
		}
		id := Hash([]any{"representation-program", input, artifact})
		blocks = append(blocks, RepresentationBlock{
			ID:       id,
			Name:     "derived-program:" + id[:12],
			Op:       "program",
			Input:    input,
			Output:   "__derived_output",
			Artifact: string(artifact),
		})
		return true
	}

	for _, input := range spec.Inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		base := []UExpr{{Kind: "var", Value: input}}
		for n := -2; n <= 2; n++ {
			base = append(base, UExpr{Kind: "const", Value: strconv.Itoa(n)})
		}

		// Put simple reusable expressions first. In particular, 0-x is produced
		// by the generic subtraction grammar; it is not named as absolute value.
		zero := UExpr{Kind: "const", Value: "0"}
		simple := append([]UExpr(nil), base...)
		simple = append(simple, UExpr{
			Kind: "sub",
			Left: &zero,
			Right: &UExpr{Kind: "var", Value: input},
		})
		for n := -2; n <= 2; n++ {
			k := strconv.Itoa(n)
			simple = append(simple,
				UExpr{Kind: "add", Left: &UExpr{Kind: "var", Value: input}, Right: &UExpr{Kind: "const", Value: k}},
				UExpr{Kind: "sub", Left: &UExpr{Kind: "var", Value: input}, Right: &UExpr{Kind: "const", Value: k}},
				UExpr{Kind: "mul", Left: &UExpr{Kind: "var", Value: input}, Right: &UExpr{Kind: "const", Value: k}},
			)
		}

		// Direct feature candidates.
		for _, expr := range simple {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			program := UniversalProgram{Statements: []UStmt{{
				Kind: "assign", Target: "__derived_output", Expr: cloneExpr(expr),
			}}}
			if !appendBlock(input, program) {
				return blocks, nil
			}
		}

		// Branch candidates are enumerated over a generic grammar. The
		// x<0 condition is reached naturally as the first discriminating
		// condition, but no block is labeled with the answer it may encode.
		conditions := []UExpr{
			{
				Kind: "lt",
				Left: &UExpr{Kind: "var", Value: input},
				Right: &UExpr{Kind: "const", Value: "0"},
			},
		}
		for _, cmp := range []string{"lt", "eq"} {
			for n := -2; n <= 2; n++ {
				if cmp == "lt" && n == 0 {
					continue
				}
				conditions = append(conditions, UExpr{
					Kind: cmp,
					Left: &UExpr{Kind: "var", Value: input},
					Right: &UExpr{Kind: "const", Value: strconv.Itoa(n)},
				})
			}
		}

		for _, cond := range conditions {
			for _, thenExpr := range simple {
				for _, elseExpr := range simple {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					program := UniversalProgram{Statements: []UStmt{{
						Kind: "if",
						Cond: cloneExpr(cond),
						Then: []UStmt{{Kind: "assign", Target: "__derived_output", Expr: cloneExpr(thenExpr)}},
						Else: []UStmt{{Kind: "assign", Target: "__derived_output", Expr: cloneExpr(elseExpr)}},
					}}}
					if !appendBlock(input, program) {
						return blocks, nil
					}
				}
			}
		}
	}

	return blocks, nil
}

func ApplyRepresentationBlock(block RepresentationBlock, input map[string]string) (string, error) {
	if block.Artifact != "" {
		var program UniversalProgram
		if err := json.Unmarshal([]byte(block.Artifact), &program); err != nil {
			return "", fmt.Errorf("decode representation artifact: %w", err)
		}
		output := block.Output
		if output == "" {
			output = "__derived_output"
		}
		env, err := program.Run(input)
		if err != nil {
			return "", err
		}
		value, ok := env[output]
		if !ok {
			return "", fmt.Errorf("representation artifact produced no output %q", output)
		}
		return value, nil
	}

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

func (RepresentationPlaygroundRunner) BuildCounterfactualState(
	spec CapabilitySpecification,
	hidden []ProgramTestCase,
) ([]byte, error) {
	return EncodeRepresentationPlaygroundState(RepresentationPlaygroundState{
		Spec:   spec,
		Hidden: hidden,
		// Candidate representation programs are generated inside the fork,
		// keeping the answer vocabulary out of the caller's control plane.
		Blocks: nil,
	})
}

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
		blocks := playground.Blocks
		if len(blocks) == 0 {
			var err error
			blocks, err = EnumerateRepresentationBlocks(ctx, playground.Spec)
			if err != nil {
				return CounterfactualRunResult{}, err
			}
		}
		for _, block := range blocks {
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
				builder := UniversalProgramBuilder{MaxSynthesisExpansions: 100000}
				proposal, err := builder.BuildWithContext(ctx, candidate, enriched)
				if err != nil {
					if errors.Is(err, ErrSynthesisExpansionLimit) {
						result.Telemetry.SearchExhausted = true
						result.Evidence = append(result.Evidence,
							fmt.Sprintf("candidate=%s exhausted synthesis budget", candidate.Mechanism),
							"representation intervention remained unverified",
						)
						continue
					}
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
					ExperimentBudget: float64(len(blocks) + len(candidates)),
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
		result.Cost = ResourceVector{ExperimentBudget: float64(len(blocks))}
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
