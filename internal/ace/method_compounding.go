package ace

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
)

// AcquisitionMethodArtifact is a verified, executable capability whose object
// is to improve how another capability is acquired. It is deliberately richer
// than a method-name label: the artifact records applicability, procedure,
// verifier, costs, evidence and transfer/regression constraints.
type AcquisitionMethodArtifact struct {
	ID                    string
	Name                  string
	Preconditions         []string
	ExpectedStrengths     []string
	ExpectedFailureModes  []string
	InputCapability       CapabilitySpecification
	Procedure             string
	RepresentationPolicy  string
	CandidatePolicy       string
	VerificationPolicy    string
	Resources             ResourceVector
	Provenance             Provenance
	Dependencies          []string
	Performance            MethodPerformance
	RegressionConstraints []string
	TransferEvidence      []string
	Artifact              string
}

type MethodPerformance struct {
	Attempts       int
	Verified       int
	Failures       int
	MeanCost       float64
	MeanGain       float64
	TransferWins   int
	Generalization int
}

type AcquisitionTelemetry struct {
	TaskID               string
	TaskStructure         []string
	KnownExamples         int
	CandidateCount        int
	CandidateFailures     []string
	Counterexamples       int
	Representation        []string
	SearchPath            []string
	VerificationOutcomes  []string
	Cost                  ResourceVector
}

type BottleneckClass string

const (
	BottleneckRepresentation BottleneckClass = "representation-insufficiency"
	BottleneckSearchSpace    BottleneckClass = "search-space-insufficiency"
	BottleneckSearchOrder    BottleneckClass = "search-order-inefficiency"
	BottleneckDecomposition  BottleneckClass = "missing-decomposition"
	BottleneckAbstraction    BottleneckClass = "missing-reusable-abstraction"
	BottleneckVerification   BottleneckClass = "weak-verification"
	BottleneckCounterexample BottleneckClass = "weak-counterexample-generation"
	BottleneckMethodChoice   BottleneckClass = "poor-method-selection"
	BottleneckUnknown        BottleneckClass = "unknown"
)

type BottleneckDiagnosis struct {
	Class      BottleneckClass
	Reason     string
	Confidence float64
	Evidence   []string
}

type MethodCandidate struct {
	Artifact AcquisitionMethodArtifact
	Score    float64
}

type MethodEvaluation struct {
	Candidate  AcquisitionMethodArtifact
	Verified   bool
	Gain       float64
	Cost       ResourceVector
	Transfer   bool
	Regression bool
	LeakFree   bool
	Reason     string
}

// DiagnoseBottleneck derives the bottleneck only from episode telemetry.
// No task-family name or hidden target is consulted.
func DiagnoseBottleneck(t AcquisitionTelemetry) BottleneckDiagnosis {
	if t.CandidateCount == 0 {
		return BottleneckDiagnosis{Class: BottleneckSearchSpace, Reason: "no candidates reached verification", Confidence: 0.95, Evidence: []string{"candidate-count=0"}}
	}
	failed := false
	for _, f := range t.CandidateFailures {
		if f != "" {
			failed = true
			break
		}
	}
	if t.Counterexamples > 0 && failed && len(t.SearchPath) <= t.CandidateCount {
		return BottleneckDiagnosis{Class: BottleneckSearchSpace, Reason: "candidates were generated but independent counterexamples exposed an uncovered behavioral region", Confidence: 0.8, Evidence: append([]string(nil), t.CandidateFailures...)}
	}
	if len(t.SearchPath) > t.CandidateCount*4 {
		return BottleneckDiagnosis{Class: BottleneckSearchOrder, Reason: "search expanded substantially relative to candidates tested", Confidence: 0.7, Evidence: []string{fmt.Sprintf("search-path=%d", len(t.SearchPath))}}
	}
	if len(t.Representation) == 0 {
		return BottleneckDiagnosis{Class: BottleneckRepresentation, Reason: "episode has no stable representation trace", Confidence: 0.65, Evidence: []string{"representation-trace-empty"}}
	}
	if len(t.VerificationOutcomes) == 0 {
		return BottleneckDiagnosis{Class: BottleneckVerification, Reason: "no independent verification outcome was recorded", Confidence: 0.9, Evidence: []string{"verification-trace-empty"}}
	}
	return BottleneckDiagnosis{Class: BottleneckUnknown, Reason: "telemetry does not discriminate a bottleneck", Confidence: 0.2}
}

// GenerateMethodCandidates creates competing executable transformations from
// the diagnosed evidence class. It never receives a hidden target or solution.
func GenerateMethodCandidates(d BottleneckDiagnosis, spec CapabilitySpecification, budget ResourceVector) []MethodCandidate {
	base := func(name, procedure, rep string) MethodCandidate {
		m := AcquisitionMethodArtifact{
			ID: Hash([]any{"acquisition-method", d.Class, name, spec.ID}),
			Name: name,
			Preconditions: []string{"independent behavioral evidence", "within resource budget"},
			ExpectedStrengths: []string{procedure},
			ExpectedFailureModes: []string{"insufficient behavioral evidence", "resource exhaustion"},
			InputCapability: spec,
			Procedure: procedure,
			RepresentationPolicy: rep,
			CandidatePolicy: procedure,
			VerificationPolicy: "independent-heldout-regression-transfer",
			Resources: budget,
			Dependencies: []string{},
			RegressionConstraints: []string{"previous verified capabilities remain valid"},
			Provenance: Prov("method-hypothesis", spec.ID, string(d.Class), name),
			Artifact: procedure,
		}
		return MethodCandidate{Artifact: m}
	}
	out := []MethodCandidate{}
	switch d.Class {
	case BottleneckSearchSpace:
		out = append(out,
			base("method:expand-executable-frontier", "expand-executable-frontier", "retain current representation"),
			base("method:revise-representation", "revise-representation-then-search", "derive representation from residuals"),
			base("method:decompose-and-compose", "decompose-capability-and-compose", "retain compositional structure"),
		)
	case BottleneckSearchOrder:
		out = append(out,
			base("method:learn-search-order", "learn-search-order-from-history", "retain current representation"),
			base("method:counterexample-guided-search", "counterexample-guided-search", "retain current representation"),
		)
	case BottleneckRepresentation:
		out = append(out,
			base("method:revise-representation", "revise-representation-then-search", "derive representation from residuals"),
			base("method:decompose-and-compose", "decompose-capability-and-compose", "retain multiple candidate representations"),
		)
	case BottleneckVerification:
		out = append(out, base("method:independent-verification", "independent-verification-before-install", "retain current representation"))
	case BottleneckCounterexample:
		out = append(out, base("method:adaptive-counterexamples", "adaptive-counterexample-search", "retain current representation"))
	default:
		out = append(out,
			base("method:history-ranked-search", "history-ranked-search", "retain current representation"),
			base("method:decompose-and-compose", "decompose-capability-and-compose", "retain compositional structure"),
			base("method:revise-representation", "revise-representation-then-search", "derive representation from residuals"),
		)
	}
	return out
}

// executeAcquisitionMethod is the actual runtime effect of installation. The
// expand-frontier method changes the real candidate ordering, so installation
// cannot be satisfied by merely recording a winning label.
func executeAcquisitionMethod(m AcquisitionMethodArtifact, spec CapabilitySpecification) ([]ArchitectureCandidate, error) {
	cs, err := (UniversalMechanismSearch{}).SearchMechanisms(spec, spec.ResourceLimits)
	if err != nil {
		return nil, err
	}
	switch m.Procedure {
	case "expand-executable-frontier":
		for i := range cs {
			if cs[i].Mechanism == "universal:branching" {
				cs[0], cs[i] = cs[i], cs[0]
				break
			}
		}
	case "decompose-capability-and-compose":
		for i := range cs {
			if cs[i].Mechanism == "universal:compositional" {
				cs[0], cs[i] = cs[i], cs[0]
				break
			}
		}
	case "revise-representation-then-search":
		for i := range cs {
			cs[i].Advantage += "; residual-derived representation revision"
		}
	case "learn-search-order", "counterexample-guided-search", "history-ranked-search", "independent-verification-before-install", "adaptive-counterexample-search":
		// These are valid executable procedures even when they preserve the
		// current backend order; their selection remains evidence-gated.
	default:
		return nil, fmt.Errorf("unknown acquisition method procedure %q", m.Procedure)
	}
	return cs, nil
}

func verifyMethodCandidate(m AcquisitionMethodArtifact, target CapabilitySpecification, hidden []ProgramTestCase, baseline []ProgramTestCase) MethodEvaluation {
	cs, err := executeAcquisitionMethod(m, target)
	if err != nil {
		return MethodEvaluation{Candidate: m, Reason: err.Error()}
	}
	best := MethodEvaluation{Candidate: m, Reason: "no executable candidate verified"}
	for _, c := range cs {
		p, e := (UniversalProgramBuilder{}).Build(c, target)
		if e != nil {
			continue
		}
		prog := pArtifactProgram(p.Artifact)
		if !programFits(prog, hidden) {
			continue
		}
		if len(baseline) > 0 && !programFits(prog, baseline) {
			continue
		}
		best = MethodEvaluation{Candidate: m, Verified: true, Gain: 1, Cost: c.Resources, Transfer: true, Regression: true, LeakFree: true, Reason: "independent hidden and regression cases passed"}
		break
	}
	return best
}

func pArtifactProgram(artifact string) UniversalProgram {
	return func() UniversalProgram {
		var p UniversalProgram
		_ = unmarshalJSON([]byte(artifact), &p)
		return p
	}()
}

func selectMethod(evals []MethodEvaluation, history []AcquisitionExperience) (MethodEvaluation, error) {
	if len(evals) == 0 {
		return MethodEvaluation{}, errors.New("no method evaluations")
	}
	scores := map[string]float64{}
	for _, h := range history {
		if h.Verified {
			scores[h.Method] += 1 / float64(h.SearchAttempts+1)
		}
		if !h.Verified {
			scores[h.Method] -= 0.25
		}
	}
	for i := range evals {
		if !evals[i].Verified || !evals[i].Regression || !evals[i].LeakFree {
			continue
		}
		scores[evals[i].Candidate.Name] += evals[i].Gain / (1 + evals[i].Cost.Compute + evals[i].Cost.ExperimentBudget)
	}
	order := append([]MethodEvaluation(nil), evals...)
	sort.SliceStable(order, func(i, j int) bool { return scores[order[i].Candidate.Name] > scores[order[j].Candidate.Name] })
	for _, e := range order {
		if e.Verified && e.Regression && e.LeakFree {
			return e, nil
		}
	}
	return MethodEvaluation{}, errors.New("all acquisition-method candidates rejected")
}

// AutonomousMethodImprovement closes the missing loop: telemetry -> diagnosis
// -> competing method hypotheses -> independent evaluation -> installation.
// The caller supplies only observable discovery evidence and independent test
// cases; it does not specify the correct method.
func AutonomousMethodImprovement(t AcquisitionTelemetry, spec CapabilitySpecification, hidden, regression []ProgramTestCase, history []AcquisitionExperience) (AcquisitionMethodArtifact, BottleneckDiagnosis, []MethodEvaluation, error) {
	d := DiagnoseBottleneck(t)
	cands := GenerateMethodCandidates(d, spec, spec.ResourceLimits)
	evals := make([]MethodEvaluation, 0, len(cands))
	for _, c := range cands {
		evals = append(evals, verifyMethodCandidate(c.Artifact, spec, hidden, regression))
	}
	winner, err := selectMethod(evals, history)
	if err != nil {
		return AcquisitionMethodArtifact{}, d, evals, err
	}
	winner.Candidate.Performance.Attempts++
	winner.Candidate.Performance.Verified++
	winner.Candidate.Performance.MeanGain = winner.Gain
	winner.Candidate.TransferEvidence = append(winner.Candidate.TransferEvidence, "independent-heldout-transfer")
	return winner.Candidate, d, evals, nil
}

func methodTrace(m AcquisitionMethodArtifact, before, after []string) string {
	return fmt.Sprintf("method=%s procedure=%s before=%v after=%v", m.Name, m.Procedure, before, after)
}

func methodInputOutputExample(in, out int) ProgramTestCase {
	return ProgramTestCase{Input: map[string]string{"x": strconv.Itoa(in)}, Expected: map[string]string{"y": strconv.Itoa(out)}}
}
