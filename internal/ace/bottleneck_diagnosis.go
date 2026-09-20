package ace

import "sort"

type BottleneckKind string

const (
	BottleneckCompute         BottleneckKind = "compute"
	BottleneckRepresentation  BottleneckKind = "representation"
	BottleneckSearch          BottleneckKind = "search"
	BottleneckEvidence        BottleneckKind = "evidence"
	BottleneckVerification    BottleneckKind = "verification"
	BottleneckInsufficient    BottleneckKind = "insufficient-evidence"
)

type BudgetTrial struct {
	Multiplier float64
	Solved     bool
}

type RepresentationAudit struct {
	CurrentFamily string
	RequiredFeatures []string
	SupportedFeatures []string
}

type FailureTelemetry struct {
	TaskID string

	StepsUsed  int
	StepLimit  int
	MemoryUsed float64
	MemoryLimit float64

	SearchExhausted bool
	SearchOrdersTested int
	AlternateOrderSolved bool

	AllCandidateFamiliesExhausted bool
	BudgetTrials []BudgetTrial

	RepresentationAudit RepresentationAudit

	CandidateReachedVerifier bool
	IndependentVerifierRejected bool

	EvidenceCount int
	MinimumEvidence int
}

type BottleneckDiagnosis struct {
	Kind BottleneckKind
	Evidence []string
	NextExperiment string
}

// DiagnoseBottleneck performs only evidence-backed classification. When the
// available telemetry cannot distinguish competing causes, it returns
// BottleneckInsufficient instead of inventing a diagnosis.
func DiagnoseBottleneck(t FailureTelemetry) BottleneckDiagnosis {
	if t.CandidateReachedVerifier && t.IndependentVerifierRejected {
		return BottleneckDiagnosis{
			Kind: BottleneckVerification,
			Evidence: []string{"candidate reached execution/acceptance stage", "independent verifier rejected the candidate"},
			NextExperiment: "construct an independently different candidate and compare verifier agreement",
		}
	}

	if t.EvidenceCount < t.MinimumEvidence {
		return BottleneckDiagnosis{
			Kind: BottleneckEvidence,
			Evidence: []string{"available evidence is below the declared minimum"},
			NextExperiment: "acquire discriminating observations before changing the acquisition mechanism",
		}
	}

	for _, trial := range t.BudgetTrials {
		if trial.Multiplier > 1 && trial.Solved {
			return BottleneckDiagnosis{
				Kind: BottleneckCompute,
				Evidence: []string{"the task fails at the baseline budget", "the task succeeds after increasing the resource budget"},
				NextExperiment: "measure the minimum resource increase required and compare against mechanism changes",
			}
		}
	}

	if t.StepsUsed >= t.StepLimit && t.StepLimit > 0 {
		return BottleneckDiagnosis{
			Kind: BottleneckCompute,
			Evidence: []string{"execution reached the declared step limit"},
			NextExperiment: "repeat under a controlled resource increase and compare outcome",
		}
	}
	if t.MemoryLimit > 0 && t.MemoryUsed >= t.MemoryLimit {
		return BottleneckDiagnosis{
			Kind: BottleneckCompute,
			Evidence: []string{"execution reached the declared memory limit"},
			NextExperiment: "repeat under a controlled memory increase and compare outcome",
		}
	}

	if t.AlternateOrderSolved {
		return BottleneckDiagnosis{
			Kind: BottleneckSearch,
			Evidence: []string{"the same candidate family succeeds under an alternate search order"},
			NextExperiment: "learn a search policy that preferentially reaches the successful region",
		}
	}

	a := t.RepresentationAudit
	if t.SearchExhausted &&
		t.SearchOrdersTested > 0 &&
		t.AllCandidateFamiliesExhausted &&
		a.CurrentFamily != "" &&
		len(a.RequiredFeatures) > 0 &&
		!containsAll(a.SupportedFeatures, a.RequiredFeatures) {
		return BottleneckDiagnosis{
			Kind: BottleneckRepresentation,
			Evidence: []string{
				"resource limits were not the observed failure boundary",
				"the current candidate families were exhausted",
				"an independent representation audit identifies required features outside the supported representation",
			},
			NextExperiment: "construct or acquire a representation that covers the missing feature set, then rerun the same task under matched resources",
		}
	}

	if t.SearchExhausted && t.SearchOrdersTested > 1 {
		return BottleneckDiagnosis{
			Kind: BottleneckSearch,
			Evidence: []string{"the existing search frontier was exhausted across multiple search orders"},
			NextExperiment: "change the search policy or generator while holding representation fixed",
		}
	}

	return BottleneckDiagnosis{
		Kind: BottleneckInsufficient,
		Evidence: []string{"telemetry does not uniquely identify compute, search, or representation as the limiting cause"},
		NextExperiment: "run a discriminating counterfactual: scale resources, vary search order, and audit representation coverage independently",
	}
}

func containsAll(have, need []string) bool {
	set := make(map[string]struct{}, len(have))
	for _, x := range have {
		set[x] = struct{}{}
	}
	for _, x := range need {
		if _, ok := set[x]; !ok {
			return false
		}
	}
	return true
}

func SortedBottleneckEvidence(d BottleneckDiagnosis) []string {
	out := append([]string(nil), d.Evidence...)
	sort.Strings(out)
	return out
}
