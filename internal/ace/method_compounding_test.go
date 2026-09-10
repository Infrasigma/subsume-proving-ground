package ace

import "testing"

func thresholdCases(threshold int, xs []int) []ProgramTestCase {
	out := make([]ProgramTestCase, 0, len(xs))
	for _, x := range xs {
		y := 0
		if x > threshold { y = 1 }
		out = append(out, methodInputOutputExample(x, y))
	}
	return out
}

func TestAutonomousMethodImprovementFromTelemetry(t *testing.T) {
	target := thresholdCases(2, []int{-3, 0, 2, 4, 7})
	hidden := thresholdCases(2, []int{-8, -1, 3, 5, 11})
	tel := AcquisitionTelemetry{
		TaskID: "opaque-task",
		TaskStructure: []string{"scalar", "conditional"},
		KnownExamples: len(target),
		CandidateCount: 7,
		CandidateFailures: []string{"independent counterexample mismatch", "search exhausted"},
		Counterexamples: 2,
		Representation: []string{"scalar-input-output"},
		SearchPath: []string{"add", "sub", "mul", "constant"},
		VerificationOutcomes: []string{"failed-heldout"},
		Cost: ResourceVector{Compute: 7, ExperimentBudget: 2},
	}
	spec := CapabilitySpecification{
		ID: "opaque-threshold",
		DesiredBehaviour: "classify input",
		Inputs: []string{"x"}, Outputs: []string{"y"},
		AcceptanceTests: []string{"heldout"},
		ResourceLimits: ResourceVector{Compute: 100, Memory: 100, TimeMS: 5000, ExperimentBudget: 20},
		KnownExamples: target,
	}
	method, diagnosis, evals, err := AutonomousMethodImprovement(tel, spec, hidden, nil, nil)
	if err != nil { t.Fatal(err) }
	if diagnosis.Class != BottleneckSearchSpace { t.Fatalf("diagnosis=%s", diagnosis.Class) }
	if len(evals) < 2 { t.Fatalf("expected competing methods, got %d", len(evals)) }
	if method.Name == "" || method.Procedure == "" || method.Artifact == "" { t.Fatal("winner is not a first-class executable method artifact") }
	if method.Procedure != "expand-executable-frontier" {
		t.Fatalf("expected evidence-driven frontier expansion to win, got %s", method.Procedure)
	}

	registry := InstalledMethodRegistry{}
	if err := registry.Install(method); err != nil { t.Fatal(err) }
	cs, err := registry.Apply(spec)
	if err != nil { t.Fatal(err) }
	if len(cs) == 0 || cs[0].Mechanism != "universal:branching" {
		t.Fatalf("installed method did not change future search order: %#v", cs)
	}
	if len(registry.Trace) < 2 { t.Fatalf("missing before/after execution trace: %#v", registry.Trace) }
	if registry.Trace[len(registry.Trace)-1] == "" { t.Fatal("empty method trace") }

	// The installed method is reused on a distinct threshold instance with a
	// different latent value; no task-specific method name is supplied.
	transfer := thresholdCases(5, []int{-4, 0, 5, 6, 13})
	transferSpec := spec
	transferSpec.ID = "opaque-threshold-transfer"
	transferSpec.KnownExamples = transfer
	transferSpec.DesiredBehaviour = "classify input under a different threshold"
	transferCS, err := registry.Apply(transferSpec)
	if err != nil { t.Fatal(err) }
	verified := false
	for _, c := range transferCS {
		p, e := (UniversalProgramBuilder{}).Build(c, transferSpec)
		if e == nil && programFits(pArtifactProgram(p.Artifact), transfer) { verified = true; break }
	}
	if !verified { t.Fatal("installed method did not transfer to structurally distinct threshold instance") }
}
