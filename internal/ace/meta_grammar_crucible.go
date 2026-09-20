package ace

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
)

// MetaGrammarTaskGenerator creates evaluator-owned procedural tasks whose
// structural depth is controlled by the seed and requested maximum depth.
type MetaGrammarTaskGenerator struct {
	Seed int64
}

// GenerateDyckTask builds a dynamic nested-bracket balancing task. Training and
// holdout examples expose only strings and labels; the generating procedure is
// not part of the capability specification.
func (g MetaGrammarTaskGenerator) GenerateDyckTask(maxDepth int) (Task, []ProgramTestCase, []ProgramTestCase, error) {
	if maxDepth < 2 {
		return Task{}, nil, nil, errors.New("dyck task requires maxDepth >= 2")
	}
	rng := rand.New(rand.NewSource(g.Seed))
	makeValid := func(depth int) string {
		var b strings.Builder
		for i := 0; i < depth; i++ {
			b.WriteByte('(')
		}
		for i := 0; i < depth; i++ {
			b.WriteByte(')')
		}
		// Add randomized balanced islands without reducing the guaranteed core
		// nesting depth.
		for i := 0; i < rng.Intn(4); i++ {
			if rng.Intn(2) == 0 {
				b.WriteString("()")
			} else {
				b.WriteString("(())")
			}
		}
		return b.String()
	}
	makeInvalid := func(s string) string {
		if len(s) == 0 {
			return ")"
		}
		mode := rng.Intn(3)
		switch mode {
		case 0:
			return s + ")"
		case 1:
			return "(" + s
		default:
			return strings.Replace(s, "()", ")( ", 1)
		}
	}
	train := make([]ProgramTestCase, 0, 8)
	hidden := make([]ProgramTestCase, 0, 6)
	for i := 0; i < 4; i++ {
		d := 2 + rng.Intn(maxDepth-1)
		train = append(train,
			ProgramTestCase{Input: map[string]string{"s": makeValid(d)}, Expected: map[string]string{"balanced": "1"}},
			ProgramTestCase{Input: map[string]string{"s": makeInvalid(makeValid(d))}, Expected: map[string]string{"balanced": "0"}},
		)
	}
	for i := 0; i < 3; i++ {
		d := maxDepth + 1 + rng.Intn(maxDepth)
		hidden = append(hidden,
			ProgramTestCase{Input: map[string]string{"s": makeValid(d)}, Expected: map[string]string{"balanced": "1"}},
			ProgramTestCase{Input: map[string]string{"s": makeInvalid(makeValid(d))}, Expected: map[string]string{"balanced": "0"}},
		)
	}
	task := Task{
		ID:           fmt.Sprintf("meta-dyck-seed-%d-depth-%d", g.Seed, maxDepth),
		Goal:         "classify whether an arbitrary-depth bracket string is balanced",
		Requirements: []string{"dynamic string scanning", "unbounded nesting state", "stack-like memory"},
		Structure:    []string{"string", "context-free", "Dyck-language", "arbitrary-depth"},
		Budget:       ResourceVector{Compute: 100, Memory: 100, TimeMS: 60000, ExperimentBudget: 50},
	}
	return task, train, hidden, nil
}

type ASTMutationProposal struct {
	ID         string
	NodeKind   string
	GrammarDelta []string
	Trigger    BottleneckDiagnosis
}

// MutateASTDefinition is the explicit escape-hatch seam. It proposes, but does
// not silently install, a new recursive/stateful grammar primitive.
func MutateASTDefinition(d BottleneckDiagnosis, spec CapabilitySpecification) (ASTMutationProposal, error) {
	if d.Class != BottleneckSearchSpace {
		return ASTMutationProposal{}, fmt.Errorf("AST mutation requires BottleneckSearchSpace, got %s", d.Class)
	}
	return ASTMutationProposal{
		ID: fmt.Sprintf("ast-mutation:%s", Hash([]any{spec.ID, d.Reason})),
		NodeKind: "recursive-stack-machine",
		GrammarDelta: []string{
			"scan-string-symbol",
			"push-stack-symbol",
			"pop-stack-symbol",
			"conditional-underflow-reject",
			"recursive-loop-until-input-exhausted",
		},
		Trigger: d,
	}, nil
}

// RunMetaGrammarCrucible executes the sealed grammar against the alien task.
// It deliberately performs the existing 50M-bounded synthesis before proposing
// any grammar mutation, preserving the failure-first scientific boundary.
func RunMetaGrammarCrucible(ctx context.Context, seed int64, maxDepth int) (BottleneckDiagnosis, ASTMutationProposal, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	task, train, hidden, err := (MetaGrammarTaskGenerator{Seed: seed}).GenerateDyckTask(maxDepth)
	if err != nil {
		return BottleneckDiagnosis{}, ASTMutationProposal{}, err
	}
	spec, err := GeneralCapabilitySpecification(task, train)
	if err != nil {
		return BottleneckDiagnosis{}, ASTMutationProposal{}, err
	}
	candidates, err := (UniversalMechanismSearch{}).SearchMechanisms(spec, spec.ResourceLimits)
	if err != nil {
		return BottleneckDiagnosis{}, ASTMutationProposal{}, err
	}
	failures := make([]string, 0, len(candidates))
	orders := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		orders = append(orders, candidate.Mechanism)
		builder := UniversalProgramBuilder{MaxSynthesisExpansions: 50_000_000}
		proposal, buildErr := builder.BuildWithContext(ctx, candidate, spec)
		if buildErr != nil {
			failures = append(failures, candidate.Mechanism+": "+buildErr.Error())
			continue
		}
		var program UniversalProgram
		if err := json.Unmarshal([]byte(proposal.Artifact), &program); err != nil {
			failures = append(failures, candidate.Mechanism+": artifact decode failure")
			continue
		}
		if err := verifyUniversalProgram(ctx, program, hidden); err != nil {
			failures = append(failures, candidate.Mechanism+": hidden verification failure")
			continue
		}
		return BottleneckDiagnosis{}, ASTMutationProposal{}, errors.New("sealed grammar unexpectedly solved the alien task")
	}
	telemetry := AcquisitionTelemetry{
		TaskID:               task.ID,
		TaskStructure:        task.Structure,
		KnownExamples:        len(train),
		CandidateCount:       len(candidates),
		CandidateFailures:    failures,
		Counterexamples:      len(hidden),
		Representation:       []string{"scalar-input-output"},
		SearchPath:            orders,
		VerificationOutcomes: []string{"all-candidates-rejected-on-alien-task"},
		Cost:                 ResourceVector{Compute: 50_000_000, ExperimentBudget: float64(len(candidates))},
	}
	diagnosis := DiagnoseAdaptiveBoundary(telemetry)
	mutation, err := MutateASTDefinition(diagnosis, spec)
	if err != nil {
		return diagnosis, ASTMutationProposal{}, fmt.Errorf("alien task reached diagnosis %s but escape hatch was not admissible: %w", diagnosis.Class, err)
	}
	return diagnosis, mutation, nil
}
