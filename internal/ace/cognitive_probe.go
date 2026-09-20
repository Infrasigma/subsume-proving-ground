package ace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// CognitiveProbeResult is a bounded empirical probe for abstraction,
// independent retention, and composition. It does not grant host capabilities.
type CognitiveProbeResult struct {
	PrimitiveAHiddenVerified bool
	PrimitiveBHiddenVerified bool
	DirectTargetSolved       bool
	ComposedTargetSolved     bool
	CompositionDepth         int
}

// RunCognitiveProbe learns two independent unary transformations from examples,
// verifies each on held-out examples, and composes them to solve a target whose
// nested control flow exceeds the direct one-branch synthesis frontier.
func RunCognitiveProbe(ctx context.Context) (CognitiveProbeResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Primitive A: absolute value.
	trainA := []ProgramTestCase{
		{Input: map[string]string{"x": "-11"}, Expected: map[string]string{"h": "11"}},
		{Input: map[string]string{"x": "-7"}, Expected: map[string]string{"h": "7"}},
		{Input: map[string]string{"x": "-3"}, Expected: map[string]string{"h": "3"}},
		{Input: map[string]string{"x": "-1"}, Expected: map[string]string{"h": "1"}},
		{Input: map[string]string{"x": "0"}, Expected: map[string]string{"h": "0"}},
		{Input: map[string]string{"x": "4"}, Expected: map[string]string{"h": "4"}},
		{Input: map[string]string{"x": "7"}, Expected: map[string]string{"h": "7"}},
	}
	hiddenA := []ProgramTestCase{
		{Input: map[string]string{"x": "-13"}, Expected: map[string]string{"h": "13"}},
		{Input: map[string]string{"x": "5"}, Expected: map[string]string{"h": "5"}},
	}


	// Primitive B: max(h, 2).
	trainB := []ProgramTestCase{
		{Input: map[string]string{"h": "0"}, Expected: map[string]string{"y": "2"}},
		{Input: map[string]string{"h": "1"}, Expected: map[string]string{"y": "2"}},
		{Input: map[string]string{"h": "5"}, Expected: map[string]string{"y": "5"}},
		{Input: map[string]string{"h": "8"}, Expected: map[string]string{"y": "8"}},
	}
	hiddenB := []ProgramTestCase{
		{Input: map[string]string{"h": "2"}, Expected: map[string]string{"y": "2"}},
		{Input: map[string]string{"h": "7"}, Expected: map[string]string{"y": "7"}},
	}



	specA, err := GeneralCapabilitySpecification(
		Task{ID: "cognition-a", Goal: "h equals abs(x)"},
		trainA,
	)
	if err != nil {
		return CognitiveProbeResult{}, err
	}
	specB, err := GeneralCapabilitySpecification(
		Task{ID: "cognition-b", Goal: "y equals max(h,2)"},
		trainB,
	)
	if err != nil {
		return CognitiveProbeResult{}, err
	}

	progA, err := learnUniversalProgram(ctx, specA)
	if err != nil {
		return CognitiveProbeResult{}, fmt.Errorf("primitive A synthesis failed: %w", err)
	}
	progB, err := learnUniversalProgram(ctx, specB)
	if err != nil {
		return CognitiveProbeResult{}, fmt.Errorf("primitive B synthesis failed: %w", err)
	}

	if err := verifyUniversalProgram(ctx, progA, hiddenA); err != nil {
		return CognitiveProbeResult{}, fmt.Errorf("primitive A hidden verification failed: %w", err)
	}
	if err := verifyUniversalProgram(ctx, progB, hiddenB); err != nil {
		return CognitiveProbeResult{}, fmt.Errorf("primitive B hidden verification failed: %w", err)
	}

	// The target requires two conditional boundaries:
	// max(abs(x), 2) has three behavioural regions.
	targetTrain := []ProgramTestCase{
		{Input: map[string]string{"x": "-5"}, Expected: map[string]string{"y": "5"}},
		{Input: map[string]string{"x": "-1"}, Expected: map[string]string{"y": "2"}},
		{Input: map[string]string{"x": "0"}, Expected: map[string]string{"y": "2"}},
		{Input: map[string]string{"x": "3"}, Expected: map[string]string{"y": "3"}},
		{Input: map[string]string{"x": "5"}, Expected: map[string]string{"y": "5"}},
	}

	targetHidden := []ProgramTestCase{
		{Input: map[string]string{"x": "-8"}, Expected: map[string]string{"y": "8"}},
		{Input: map[string]string{"x": "1"}, Expected: map[string]string{"y": "2"}},
		{Input: map[string]string{"x": "9"}, Expected: map[string]string{"y": "9"}},
	}

	targetSpec, err := GeneralCapabilitySpecification(
		Task{ID: "cognition-target", Goal: "y equals max(abs(x),2)"},
		targetTrain,
	)
	if err != nil {
		return CognitiveProbeResult{}, err
	}

	directSolved := false
	if direct, buildErr := learnUniversalProgram(ctx, targetSpec); buildErr == nil {
		directSolved = verifyUniversalProgram(ctx, direct, targetHidden) == nil
	}

	composed, err := composeUnaryUniversalPrograms(progA, "x", "h", progB, "h")
	if err != nil {
		return CognitiveProbeResult{}, err
	}
	if err := verifyUniversalProgram(ctx, composed, targetHidden); err != nil {
		return CognitiveProbeResult{}, fmt.Errorf("composed target failed hidden verification: %w", err)
	}

	return CognitiveProbeResult{
		PrimitiveAHiddenVerified: true,
		PrimitiveBHiddenVerified: true,
		DirectTargetSolved:       directSolved,
		ComposedTargetSolved:     true,
		CompositionDepth:         2,
	}, nil
}

func learnUniversalProgram(ctx context.Context, spec CapabilitySpecification) (UniversalProgram, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	candidates, err := (UniversalMechanismSearch{}).SearchMechanisms(spec, spec.ResourceLimits)
	if err != nil {
		return UniversalProgram{}, err
	}
	const cognitiveProbeSynthesisBudget = 50_000_000

	builder := UniversalProgramBuilder{MaxSynthesisExpansions: cognitiveProbeSynthesisBudget}
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return UniversalProgram{}, err
		}
		proposal, err := builder.BuildWithContext(ctx, candidate, spec)
		if err != nil {
			continue
		}
		var program UniversalProgram
		if err := json.Unmarshal([]byte(proposal.Artifact), &program); err != nil {
			continue
		}
		if !programFits(program, spec.KnownExamples) {
			continue
		}
		return program, nil
	}
	return UniversalProgram{}, errors.New("no universal program matched training examples")
}

func verifyUniversalProgram(ctx context.Context, program UniversalProgram, cases []ProgramTestCase) error {
	for _, tc := range cases {
		if err := ctx.Err(); err != nil {
			return err
		}
		got, err := program.Run(tc.Input)
		if err != nil {
			return err
		}
		for key, expected := range tc.Expected {
			if got[key] != expected {
				return fmt.Errorf("%s: got %q want %q", key, got[key], expected)
			}
		}
	}
	return nil
}

func composeUnaryUniversalPrograms(
	first UniversalProgram,
	firstInput, firstOutput string,
	second UniversalProgram,
	secondInput string,
) (UniversalProgram, error) {
	if firstInput == "" || firstOutput == "" || secondInput == "" {
		return UniversalProgram{}, errors.New("composition roles must be non-empty")
	}
	if len(first.Statements) == 0 || len(second.Statements) == 0 {
		return UniversalProgram{}, errors.New("cannot compose empty programs")
	}

	out := UniversalProgram{Statements: make([]UStmt, 0, len(first.Statements)+len(second.Statements))}
	for _, stmt := range first.Statements {
		out.Statements = append(out.Statements, cloneStmt(stmt))
	}
	for _, stmt := range second.Statements {
		out.Statements = append(out.Statements, renameExprInput(stmt, secondInput, firstOutput))
	}
	return out, nil
}

func cloneStmt(s UStmt) UStmt {
	out := s
	if s.Expr != nil {
		out.Expr = cloneExpr(*s.Expr)
	}
	if s.Cond != nil {
		out.Cond = cloneExpr(*s.Cond)
	}
	if s.Then != nil {
		out.Then = make([]UStmt, len(s.Then))
		for i := range s.Then {
			out.Then[i] = cloneStmt(s.Then[i])
		}
	}
	if s.Else != nil {
		out.Else = make([]UStmt, len(s.Else))
		for i := range s.Else {
			out.Else[i] = cloneStmt(s.Else[i])
		}
	}
	if s.Body != nil {
		out.Body = make([]UStmt, len(s.Body))
		for i := range s.Body {
			out.Body[i] = cloneStmt(s.Body[i])
		}
	}
	return out
}

func renameExprInput(s UStmt, from, to string) UStmt {
	out := cloneStmt(s)
	out.Expr = renameExprVar(out.Expr, from, to)
	out.Cond = renameExprVar(out.Cond, from, to)
	for i := range out.Then {
		out.Then[i] = renameExprInput(out.Then[i], from, to)
	}
	for i := range out.Else {
		out.Else[i] = renameExprInput(out.Else[i], from, to)
	}
	for i := range out.Body {
		out.Body[i] = renameExprInput(out.Body[i], from, to)
	}
	return out
}

func renameExprVar(e *UExpr, from, to string) *UExpr {
	if e == nil {
		return nil
	}
	out := cloneExpr(*e)
	if out.Kind == "var" && out.Value == from {
		out.Value = to
	}
	return out
}
