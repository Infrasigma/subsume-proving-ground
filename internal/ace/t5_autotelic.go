package ace

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	AutotelicGapStringReverseUpperVowels = "string.reverse-then-uppercase-vowels"
	AutotelicTaskFamily                 = "autotelic-string-composition"
	AutotelicTaskID                     = "06-autotelic-reverse-uppercase-vowels"
)

var ErrNoAutotelicGap = errors.New("autotelic task generator found no verified capability gap")

// AutotelicTaskBundle keeps evaluator-owned holdouts out of ReactorTask.
// Search receives only Bundle.Task; Bundle.hidden is consumed only by the
// verifier wrapper created at the moment the generated task is executed.
type AutotelicTaskBundle struct {
	Task          ReactorTask
	BoundaryScore int
	Complexity    int
	GapClass      string
	hidden        []ReactorExample
}

type AutotelicTaskGenerator struct {
	// The generator is deliberately one complexity step ahead of the current
	// verified boundary. A larger jump would no longer be a "slightly beyond"
	// self-directed task.
	ComplexityDelta int
}

func DefaultAutotelicTaskGenerator() *AutotelicTaskGenerator {
	return &AutotelicTaskGenerator{ComplexityDelta: 1}
}

func (g AutotelicTaskGenerator) Generate(ctx context.Context, lib *AbstractionLibrary, heuristic *SearchHeuristicProgram) (AutotelicTaskBundle, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return AutotelicTaskBundle{}, err
	}
	if lib == nil {
		return AutotelicTaskBundle{}, errors.New("autotelic task generation requires the hydrated abstraction library")
	}
	if heuristic == nil {
		return AutotelicTaskBundle{}, errors.New("autotelic task generation requires the active search heuristic")
	}
	if err := heuristic.Validate(); err != nil {
		return AutotelicTaskBundle{}, fmt.Errorf("active search heuristic is invalid: %w", err)
	}
	delta := g.ComplexityDelta
	if delta <= 0 {
		delta = 1
	}
	gap := AutotelicGapStringReverseUpperVowels
	if autotelicGapAlreadyAdmitted(lib, gap) {
		return AutotelicTaskBundle{}, ErrNoAutotelicGap
	}

	boundary := autotelicCapabilityBoundary(lib, heuristic)
	complexity := boundary + delta
	if complexity != boundary+1 {
		return AutotelicTaskBundle{}, fmt.Errorf("autotelic complexity jump must be exactly one: boundary=%d complexity=%d", boundary, complexity)
	}

	public := []ReactorExample{
		{Input: []string{"hello world"}, Expected: []string{"dlrOw OllEh"}},
		{Input: []string{"ace reactor"}, Expected: []string{"rOtcAEr EcA"}},
		{Input: []string{"strict verification"}, Expected: []string{"nOItAcIfIrEv tcIrts"}},
	}
	hidden := autotelicHiddenExamples(heuristicSignature(*heuristic), lib)
	if !autotelicExamplesAreDisjoint(public, hidden) {
		return AutotelicTaskBundle{}, errors.New("autotelic generator produced overlapping public and hidden examples")
	}
	for _, example := range public {
		if example.Input[0] == example.Expected[0] {
			return AutotelicTaskBundle{}, errors.New("autotelic complexity gate rejected identity example")
		}
	}
	for _, example := range hidden {
		if example.Input[0] == example.Expected[0] {
			return AutotelicTaskBundle{}, errors.New("autotelic hidden complexity gate rejected identity example")
		}
	}

	task := ReactorTask{
		ID:                 AutotelicTaskID,
		Family:             AutotelicTaskFamily,
		InputKind:          "string",
		Description:        "Autonomously bridge two verified string capabilities by composing reversal with vowel uppercasing. The evaluator-generated holdout remains hidden from search.",
		Examples:           public,
		MaxSearchDepth:     1,
		MinProcedureSteps:  1,
		AdmitAsAbstraction: true,
		Autotelic:          true,
		ComplexityScore:    complexity,
		GapClass:           gap,
		Budget: ResourceVector{
			Compute:          500,
			Memory:           256,
			Storage:          64,
			TimeMS:            10000,
			ExperimentBudget: 256,
		},
	}
	if err := task.Validate(); err != nil {
		return AutotelicTaskBundle{}, fmt.Errorf("generated autotelic task failed validation: %w", err)
	}
	return AutotelicTaskBundle{
		Task:          task,
		BoundaryScore: boundary,
		Complexity:    complexity,
		GapClass:      gap,
		hidden:        append([]ReactorExample(nil), hidden...),
	}, nil
}

func (a AcquiredAbstraction) GapClass() string {
	const prefix = "autotelic:"
	if !strings.HasPrefix(a.Name, prefix) {
		return ""
	}
	return strings.TrimPrefix(a.Name, prefix)
}

func autotelicCapabilityBoundary(lib *AbstractionLibrary, heuristic *SearchHeuristicProgram) int {
	boundary := 0
	if heuristic != nil {
		// T4 provides a verified bounded search controller.
		boundary = 1
	}
	for _, abstraction := range lib.Abstractions {
		if abstraction.ArtifactType == SynthesizedProgramArtifactType && abstraction.SynthesizedProgram != nil {
			if boundary < 1 {
				boundary = 1
			}
		}
		if abstraction.GapClass() != "" {
			if boundary < 2 {
				boundary = 2
			}
		}
	}
	return boundary
}

func autotelicGapAlreadyAdmitted(lib *AbstractionLibrary, gap string) bool {
	for _, abstraction := range lib.Abstractions {
		if abstraction.GapClass() == gap {
			return true
		}
	}
	return false
}

func autotelicHiddenExamples(heuristicSignature string, lib *AbstractionLibrary) []ReactorExample {
	pools := [][]ReactorExample{
		{
			{Input: []string{"autonomous expansion"}, Expected: []string{"nOIsnApxE sUOmOnOtUA"}},
			{Input: []string{"cryptographic learning"}, Expected: []string{"gnInrAEl cIhpArgOtpyrc"}},
			{Input: []string{"epistemic boundary"}, Expected: []string{"yrAdnUOb cImEtsIpE"}},
		},
		{
			{Input: []string{"verified recursion"}, Expected: []string{"nOIsrUcEr dEIfIrEv"}},
			{Input: []string{"capability ledger"}, Expected: []string{"rEgdEl ytIlIbApAc"}},
			{Input: []string{"deterministic search"}, Expected: []string{"hcrAEs cItsInImrEtEd"}},
		},
	}
	selector := 0
	for _, r := range heuristicSignature {
		selector = (selector*33 + int(r)) % len(pools)
	}
	selector = (selector + len(lib.Abstractions)) % len(pools)
	return append([]ReactorExample(nil), pools[selector]...)
}

func autotelicExamplesAreDisjoint(public, hidden []ReactorExample) bool {
	seen := make(map[string]struct{}, len(public))
	for _, example := range public {
		if len(example.Input) != 1 || len(example.Expected) != 1 {
			return false
		}
		seen[example.Input[0]] = struct{}{}
	}
	for _, example := range hidden {
		if len(example.Input) != 1 || len(example.Expected) != 1 {
			return false
		}
		if _, exists := seen[example.Input[0]]; exists {
			return false
		}
	}
	return true
}

type autotelicHiddenVerifier struct {
	delegate ReactorVerifier
	hidden   map[string][]ReactorExample
}

func newAutotelicHiddenVerifier(delegate ReactorVerifier, bundle AutotelicTaskBundle) ReactorVerifier {
	return &autotelicHiddenVerifier{
		delegate: delegate,
		hidden:   map[string][]ReactorExample{bundle.Task.ID: append([]ReactorExample(nil), bundle.hidden...)},
	}
}

func (v *autotelicHiddenVerifier) Verify(ctx context.Context, task ReactorTask, procedure AcquisitionProcedure, lib *AbstractionLibrary) error {
	if examples, ok := v.hidden[task.ID]; ok {
		if err := ctx.Err(); err != nil {
			return err
		}
		if procedureFitsReactorExamples(procedure, examples, lib) {
			return nil
		}
		return errors.New("autotelic hidden verification rejected candidate")
	}
	if v.delegate == nil {
		return errors.New("autotelic verifier has no delegated evaluator")
	}
	return v.delegate.Verify(ctx, task, procedure, lib)
}

func (v *autotelicHiddenVerifier) VerifySynthesizedProgram(ctx context.Context, task ReactorTask, program SynthesizedProgram) error {
	if examples, ok := v.hidden[task.ID]; ok {
		return verifySynthesizedProgramExamples(ctx, examples, program)
	}
	delegate, ok := v.delegate.(SynthesizedProgramReactorVerifier)
	if !ok {
		return fmt.Errorf("delegated verifier does not support synthesized-program verification")
	}
	return delegate.VerifySynthesizedProgram(ctx, task, program)
}

func verifySynthesizedProgramExamples(ctx context.Context, examples []ReactorExample, program SynthesizedProgram) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(examples) < 2 {
		return errors.New("autotelic evaluator requires at least two hidden examples")
	}
	if err := program.Validate(); err != nil {
		return fmt.Errorf("autotelic synthesized program rejected before hidden verification: %w", err)
	}
	for _, example := range examples {
		if len(example.Input) != 1 || len(example.Expected) != 1 {
			return errors.New("autotelic hidden fixture must contain one input and one expected output")
		}
		got, err := program.Execute(ctx, example.Input[0])
		if err != nil {
			return fmt.Errorf("autotelic hidden execution failed: %w", err)
		}
		if got != example.Expected[0] {
			return fmt.Errorf("autotelic hidden verification mismatch: got %q want %q", got, example.Expected[0])
		}
	}
	return nil
}
