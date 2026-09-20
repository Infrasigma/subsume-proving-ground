package ace

import (
	"fmt"
	"strings"
)

// FailureTelemetry is the diagnosis-domain evidence model. It is deliberately
// richer than AcquisitionTelemetry: execution traces are projected into this
// schema before bottleneck classification.
type FailureTelemetry struct {
	TaskID string

	StepsUsed  int
	StepLimit  int
	MemoryUsed float64
	MemoryLimit float64
	BudgetTrials []BudgetTrial

	SearchExhausted bool
	SearchAttempts int
	SearchOrdersTested int
	AlternateOrderSolved bool
	AllCandidateFamiliesExhausted bool

	CurrentRepresentation string
	RequiredRepresentationFeatures []string
	SupportedRepresentationFeatures []string

	CandidateReachedVerifier bool
	IndependentVerifierRejected bool

	EvidenceCount int
	MinimumEvidence int
}

type BudgetTrial struct {
	Multiplier float64
	Solved bool
}

// ProjectAcquisitionTelemetry is an explicit lossy boundary between execution
// evidence and diagnosis evidence. It never invents resource or representation
// facts that are not present in the source telemetry.
func ProjectAcquisitionTelemetry(t AcquisitionTelemetry) FailureTelemetry {
	f := FailureTelemetry{
		TaskID: t.TaskID,
		SearchAttempts: t.CandidateCount,
		EvidenceCount: t.KnownExamples + t.Counterexamples,
		MinimumEvidence: 1,
		CandidateReachedVerifier: len(t.VerificationOutcomes) > 0,
		IndependentVerifierRejected: verificationRejected(t.VerificationOutcomes),
		CurrentRepresentation: strings.Join(t.Representation, "|"),
	}

	f.SearchExhausted = t.CandidateCount == 0 ||
		(t.CandidateCount > 0 && len(t.CandidateFailures) >= t.CandidateCount)
	f.AllCandidateFamiliesExhausted = f.SearchExhausted && t.CandidateCount > 0
	if len(t.SearchPath) > 0 {
		f.SearchOrdersTested = 1
	}
	return f
}

func DiagnoseFailureTelemetry(t FailureTelemetry) BottleneckDiagnosis {
	// An explicitly exhausted candidate frontier takes precedence over a
	// minority verifier rejection so one overfit candidate cannot mask a
	// distribution-level search-space failure.
	if t.SearchExhausted && t.AllCandidateFamiliesExhausted {
		return BottleneckDiagnosis{
			Class: BottleneckSearchSpace,
			Reason: "candidate frontier exhausted across all declared candidate families",
			Confidence: 0.95,
			Evidence: []string{"search frontier exhausted", "all candidate families exhausted"},
		}
	}

	if t.CandidateReachedVerifier && t.IndependentVerifierRejected {
		return BottleneckDiagnosis{
			Class: BottleneckVerification,
			Reason: "candidate reached independent verification and was rejected",
			Confidence: 0.95,
			Evidence: []string{"verifier reached", "independent verifier rejected candidate"},
		}
	}

	if t.EvidenceCount < t.MinimumEvidence {
		return BottleneckDiagnosis{
			Class: BottleneckUnknown,
			Reason: "insufficient discriminating evidence",
			Confidence: 0.98,
			Evidence: []string{"evidence-count below minimum"},
		}
	}

	for _, trial := range t.BudgetTrials {
		if trial.Multiplier > 1 && trial.Solved {
			return BottleneckDiagnosis{
				Class: BottleneckResource,
				Reason: "failure disappears only under increased resource budget; resource is the supported explanation boundary",
				Confidence: 0.90,
				Evidence: []string{"baseline budget failed", "larger controlled budget succeeded"},
			}
		}
	}

	if t.StepsUsed >= t.StepLimit && t.StepLimit > 0 {
		return BottleneckDiagnosis{
			Class: BottleneckResource,
			Reason: "execution reached the declared step limit",
			Confidence: 0.90,
			Evidence: []string{"step limit reached"},
		}
	}

	if t.MemoryLimit > 0 && t.MemoryUsed >= t.MemoryLimit {
		return BottleneckDiagnosis{
			Class: BottleneckResource,
			Reason: "execution reached the declared memory limit",
			Confidence: 0.90,
			Evidence: []string{"memory limit reached"},
		}
	}

	if t.AlternateOrderSolved {
		return BottleneckDiagnosis{
			Class: BottleneckSearchOrder,
			Reason: "same mechanism family succeeds under a different search order",
			Confidence: 0.90,
			Evidence: []string{"alternate search order solved task"},
		}
	}

	if t.SearchExhausted &&
		t.AllCandidateFamiliesExhausted &&
		t.CurrentRepresentation != "" &&
		len(t.RequiredRepresentationFeatures) > 0 &&
		!containsAllStrings(t.SupportedRepresentationFeatures, t.RequiredRepresentationFeatures) {
		return BottleneckDiagnosis{
			Class: BottleneckRepresentation,
			Reason: "candidate families are exhausted and an independently identified required feature is outside the supported representation",
			Confidence: 0.90,
			Evidence: []string{
				"search frontier exhausted",
				"candidate families exhausted",
				"required representation features exceed supported features",
			},
		}
	}

	if t.SearchExhausted && t.SearchOrdersTested > 1 {
		return BottleneckDiagnosis{
			Class: BottleneckSearchSpace,
			Reason: "search frontier was exhausted across multiple search orders",
			Confidence: 0.75,
			Evidence: []string{"multiple search orders exhausted"},
		}
	}

	return BottleneckDiagnosis{
		Class: BottleneckUnknown,
		Reason: "telemetry does not uniquely identify the limiting mechanism",
		Confidence: 0.20,
		Evidence: []string{"discriminating experiment required"},
	}
}

// DiagnoseAdaptiveBoundary first consumes the separated diagnosis schema. When
// that projection is insufficient, it falls back to the legacy acquisition
// telemetry diagnosis rather than fabricating certainty.
// DiagnoseBottleneck preserves the historical acquisition-telemetry entry point.
// New code should prefer DiagnoseAdaptiveBoundary or DiagnoseFailureTelemetry so
// the evidence domain is explicit, but existing scientific controls must remain
// source-compatible while the diagnosis schema is being migrated.
func DiagnoseBottleneck(t AcquisitionTelemetry) BottleneckDiagnosis {
	return DiagnoseAcquisitionTelemetry(t)
}

func DiagnoseAdaptiveBoundary(t AcquisitionTelemetry) BottleneckDiagnosis {
	projected := ProjectAcquisitionTelemetry(t)
	d := DiagnoseFailureTelemetry(projected)
	if d.Class != BottleneckUnknown {
		return d
	}
	return DiagnoseAcquisitionTelemetry(t)
}

func verificationRejected(outcomes []string) bool {
	for _, outcome := range outcomes {
		s := strings.ToLower(outcome)
		if strings.Contains(s, "reject") ||
			strings.Contains(s, "fail") ||
			strings.Contains(s, "invalid") {
			return true
		}
	}
	return false
}

func containsAllStrings(have, need []string) bool {
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

func FailureTelemetrySummary(t FailureTelemetry) string {
	return fmt.Sprintf(
		"task=%s search_exhausted=%t candidate_families_exhausted=%t representation=%q verifier_rejected=%t evidence=%d/%d",
		t.TaskID,
		t.SearchExhausted,
		t.AllCandidateFamiliesExhausted,
		t.CurrentRepresentation,
		t.IndependentVerifierRejected,
		t.EvidenceCount,
		t.MinimumEvidence,
	)
}
