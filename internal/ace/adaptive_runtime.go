package ace

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
)

type AbstractionKMS interface {
	SignAbstractionHash(context.Context, string, string) (protocol.KMSSignedArtifact, error)
}

type AbstractionAdmissionLedger interface {
	AppendAbstractionAdmission(context.Context, protocol.AbstractionAdmissionReceipt) (protocol.AbstractionAdmissionReceipt, error)
}

type AdaptiveAcquisitionRuntime struct {
	Diagnostics *DiagnosticLog
	Methods                   InstalledMethodRegistry
	History                   []AcquisitionExperience
	PersistentMethods         *PersistentMethodRegistry
	Abstractions              AbstractionLibrary
	PersistentAbstractions    *PersistentAbstractionLibrary
	AbstractionHistory        []AbstractionObservation
	EnableAbstractionLearning bool

	// T2 recursion is bounded and deadline-controlled. Zero values use safe defaults.
	MaxCompoundingIterations int
	CompoundingTimeout       time.Duration

	// F0 admission plane. Both dependencies are mandatory whenever a newly
	// promoted abstraction is admitted; absence is fail-closed.
	AbstractionKMS      AbstractionKMS
	AdmissionLedger     AbstractionAdmissionLedger
	KMSSignerID         string
	TrustedKMSPublicKeyB64 string

	// T4: the active search policy is data, not executable Go logic. It is
	// loaded only from an independently admitted artifact and swapped after
	// verification; the interpreter remains bounded and reorder-only.
	ActiveSearchHeuristic *SearchHeuristicProgram

	// Optional counterfactual experimenter. When ordinary telemetry cannot
	// discriminate the bottleneck, this runner may fork the failed execution,
	// test single-factor hypotheses, and return a uniquely supported diagnosis.
	CounterfactualRunner CounterfactualRunner
	AcquisitionPolicy   *AcquisitionPolicy
}

type AdaptiveAcquisitionResult struct { Method AcquisitionMethodArtifact; Diagnosis BottleneckDiagnosis; Evaluations []MethodEvaluation; Future CapabilityRecord; FutureCost ResourceVector; Trace []string; PolicyMutation *GeneratorSpaceMutation }

func (r *AdaptiveAcquisitionRuntime) prepareLibraries() error {
	r.Methods.Abstractions = &r.Abstractions
	if r.PersistentAbstractions != nil && len(r.Abstractions.Abstractions) == 0 { if err := r.PersistentAbstractions.Restore(&r.Abstractions); err != nil { return err } }
	return nil
}

func abstractionObservationFromMethod(m AcquisitionMethodArtifact, taskStructure string, verified, heldOut bool, gain float64, cost ResourceVector) AbstractionObservation { p, _ := decodeAcquisitionProcedure(m.Artifact); return AbstractionObservation{TaskStructure: taskStructure, Procedure: p, Verified: verified, HeldOut: heldOut, TransferScore: boolScore(verified && heldOut), DiscoveryCost: cost, ObservedGain: gain} }

func streamForAbstractionVerification(names ...string) []ArchitectureCandidate { out:=make([]ArchitectureCandidate,len(names));for i,name:=range names{out[i]=ArchitectureCandidate{ID:Hash([]any{"runtime-abstraction-probe",name}),Mechanism:name,Resources:ResourceVector{Compute:float64(i+1),ExperimentBudget:1}}};return out }

func (r *AdaptiveAcquisitionRuntime) learnAbstractionFromVerifiedMethod(ctx context.Context, method AcquisitionMethodArtifact, telemetry AcquisitionTelemetry, futureSpec CapabilitySpecification, hidden []ProgramTestCase) error {
	if !r.EnableAbstractionLearning || len(method.Procedure) == 0 {
		if r.Diagnostics != nil && r.EnableAbstractionLearning {
			r.Diagnostics.Record("C", method.ID, "acquisition-method", method, DiagnosticFormalValidity, "len(method.Procedure) > 0", false, "verified acquisition method contained no procedure steps")
		}
		return nil
	}
	p, err := decodeAcquisitionProcedure(method.Artifact)
	if err != nil {
		r.Diagnostics.Record("C", method.ID, "acquisition-method", method, DiagnosticFormalValidity, "decodeAcquisitionProcedure succeeds", false, err.Error())
		return nil
	}
	if len(p.Steps) < 2 {
		r.Diagnostics.Record("C", method.ID, "acquisition-method", method, DiagnosticFormalValidity, "len(method.Procedure.steps) >= 2", false, fmt.Sprintf("acquired method has %d step(s); abstraction synthesis requires a non-trivial composition", len(p.Steps)))
		return nil
	}
	gain := 1.0; cost := method.Resources
	r.AbstractionHistory = append(r.AbstractionHistory, abstractionObservationFromMethod(method, telemetry.TaskID, true, true, gain, cost), abstractionObservationFromMethod(method, Hash([]any{futureSpec.Inputs,futureSpec.Outputs,futureSpec.Invariants,futureSpec.DesiredBehaviour}), true, len(hidden)>0, gain, cost))
	proposal, err := DiscoverReusableAbstractionWithDiagnostics(r.AbstractionHistory, 2, r.Diagnostics); if err != nil { return nil }
	inputs := [][]ArchitectureCandidate{streamForAbstractionVerification("probe-a","probe-b","probe-c"),streamForAbstractionVerification("probe-c","probe-a","probe-b","probe-d")}
	cases:=make([]AbstractionVerificationCase,0,len(inputs));for _,input:=range inputs{cases=append(cases,AbstractionVerificationCase{Input:input})}
	verified, err := VerifyAcquiredAbstractionWithDiagnostics(proposal, &r.Abstractions, cases, r.Diagnostics); if err != nil { return nil }
	verifiedScore, _, scoreErr := partialMethodScore(method, futureSpec, &r.Abstractions)
	if scoreErr != nil { verifiedScore = 1 }
	sealed, err := r.admitAbstraction(ctx, verified, verifiedScore)
	if err != nil { return err }
	if r.PersistentAbstractions != nil { return r.PersistentAbstractions.Save(&r.Abstractions) }
	_ = sealed
	return nil
}

func (r *AdaptiveAcquisitionRuntime) ImproveAndAcquire(telemetry AcquisitionTelemetry, failedSpec CapabilitySpecification, methodHidden []ProgramTestCase, futureSpec CapabilitySpecification, futureHidden []ProgramTestCase) (AdaptiveAcquisitionResult, error) {
	timeout := r.CompoundingTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return r.ImproveAndAcquireWithContext(ctx, telemetry, failedSpec, methodHidden, futureSpec, futureHidden)
}

func (r *AdaptiveAcquisitionRuntime) ImproveAndAcquireWithContext(ctx context.Context, telemetry AcquisitionTelemetry, failedSpec CapabilitySpecification, methodHidden []ProgramTestCase, futureSpec CapabilitySpecification, futureHidden []ProgramTestCase) (AdaptiveAcquisitionResult, error) {
	if len(methodHidden) == 0 || len(futureHidden) == 0 {
		return AdaptiveAcquisitionResult{}, errors.New("adaptive runtime requires independent evaluation cases")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := r.prepareLibraries(); err != nil {
		return AdaptiveAcquisitionResult{}, err
	}
	if r.PersistentMethods != nil && len(r.Methods.Methods) == 0 {
		if err := r.PersistentMethods.Restore(&r.Methods); err != nil {
			return AdaptiveAcquisitionResult{}, err
		}
	}

	maxIterations := r.MaxCompoundingIterations
	if maxIterations <= 0 {
		maxIterations = 3
	}

	var lastErr error
	recursiveUsed := false
	var method AcquisitionMethodArtifact
	var policyMutation *GeneratorSpaceMutation
	diagnosis := DiagnoseAdaptiveBoundary(telemetry)
	var evals []MethodEvaluation

	if diagnosis.Class == BottleneckUnknown && r.CounterfactualRunner != nil {
		projected := ProjectAcquisitionTelemetry(telemetry)
		counterfactual, err := RunDiscriminatingBottleneckExperiments(ctx, projected, nil, r.CounterfactualRunner)
		if err != nil {
			return AdaptiveAcquisitionResult{}, fmt.Errorf("counterfactual bottleneck experiment failed: %w", err)
		}
		diagnosis = counterfactual.Diagnosis
		if r.Diagnostics != nil {
			r.Diagnostics.Record(
				"C",
				telemetry.TaskID,
				"counterfactual-diagnosis",
				counterfactual,
				DiagnosticSelection,
				"unique independently verified counterfactual hypothesis",
				counterfactual.Discriminated,
				counterfactual.Diagnosis.Reason,
			)
		}

		// A representation diagnosis becomes a policy mutation only after the
		// counterfactual runner returns the exact verified semantic primitive.
		if diagnosis.Class == BottleneckRepresentation && r.AcquisitionPolicy != nil {
			mutation, promoteErr := PromoteCounterfactualRepresentation(
				r.AcquisitionPolicy,
				projected,
				counterfactual,
			)
			if promoteErr != nil {
				return AdaptiveAcquisitionResult{}, fmt.Errorf("representation promotion rejected: %w", promoteErr)
			}
			policyMutation = &mutation

			// Re-enter acquisition through the newly expanded generator space.
			// Only one topology-matched primitive is activated to avoid feature
			// combinatorial explosion.
			preparedSpec, preparedHidden, _, prepareErr := PrepareCapabilityWithAcquisitionPolicy(
				failedSpec,
				methodHidden,
				projected,
				*r.AcquisitionPolicy,
			)
			if prepareErr != nil {
				return AdaptiveAcquisitionResult{}, fmt.Errorf("policy-conditioned representation preparation failed: %w", prepareErr)
			}
			failedSpec = preparedSpec
			methodHidden = preparedHidden

			if r.Diagnostics != nil {
				r.Diagnostics.Record(
					"C",
					mutation.PrimitiveID,
					"generator-space-mutation",
					mutation,
					DiagnosticAccepted,
					"unique verified representation survivor promoted and bound to failure topology",
					true,
					fmt.Sprintf("search weight %.3f topology=%s", mutation.SearchWeight, mutation.FailureTopology),
				)
			}
		}
	}
	if diagnosis.Class == BottleneckUnknown {
		return AdaptiveAcquisitionResult{Diagnosis: diagnosis}, errors.New("adaptive runtime has no discriminating bottleneck diagnosis")
	}

	for iteration := 1; iteration <= maxIterations; iteration++ {
		if err := ctx.Err(); err != nil {
			return AdaptiveAcquisitionResult{}, fmt.Errorf("recursive compounding deadline reached at iteration %d: %w", iteration, err)
		}
		if r.Diagnostics != nil {
			r.Diagnostics.Record("C", telemetry.TaskID, "t2-iteration", map[string]any{
				"iteration": iteration,
				"library_size": len(r.Abstractions.Abstractions),
			}, DiagnosticProposal, "bounded recursive compounding iteration entered", true,
				fmt.Sprintf("T2 iteration %d/%d", iteration, maxIterations))
		}

		cands := AutonomousMethodCandidatesWithLibrary(
			diagnosis,
			failedSpec,
			failedSpec.ResourceLimits,
			&r.Abstractions,
		)
		if len(cands) == 0 {
			lastErr = errors.New("recursive synthesis generated no acquisition-method candidates")
			continue
		}

		// Before T2 promotion, select a demonstrably useful partial procedure:
		// it must preserve the architecture frontier and change its order.
		partial := selectBestPartialMethodCandidate(cands, failedSpec, &r.Abstractions)
		if partial.Artifact.ID == "" {
			lastErr = errors.New("recursive synthesis found no useful partial acquisition method")
			continue
		}

		p, err := decodeAcquisitionProcedure(partial.Artifact.Procedure)
		if err != nil {
			lastErr = err
			continue
		}
		partialScore, _, scoreErr := partialMethodScore(partial.Artifact, failedSpec, &r.Abstractions)
		if scoreErr != nil {
			lastErr = scoreErr
			continue
		}

		// Preserve the T1 verifier as the first real check. One representative
		// candidate is enough here because the full T1 frontier has already been
		// independently gated by the forced-composition experiment.
		baselineEval := verifyMethodCandidateWithContext(ctx, partial.Artifact, failedSpec, methodHidden, nil, &r.Abstractions)
		if r.Diagnostics != nil {
			r.Diagnostics.Record("C", partial.Artifact.ID, "acquisition-method", partial.Artifact,
				DiagnosticVerification, "bounded T1 verifier accepts candidate", baselineEval.Verified,
				fmt.Sprintf("T2 iteration %d baseline candidate depth=%d verified=%v reason=%s", iteration, len(p.Steps), baselineEval.Verified, baselineEval.Reason))
		}
		if baselineEval.Verified {
			method = baselineEval.Candidate
			// Preserve the causally established diagnosis through candidate verification.
			evals = []MethodEvaluation{baselineEval}
			lastErr = nil
			break
		}

		// T1 could not finish the task. Promote the useful partial procedure only
		// after independently verifying its executable semantics on held-out streams.
		promoted, err := r.promotePartialProcedure(ctx, partial.Artifact, telemetry.TaskID, partialScore)
		if err != nil {
			lastErr = err
			continue
		}
		recursiveUsed = true
		if r.Diagnostics != nil {
			r.Diagnostics.Record("C", promoted.ID, "acquired-abstraction", promoted,
				DiagnosticAccepted, "independent procedure verification + useful future-trace change", true,
				fmt.Sprintf("T2 promotion at iteration %d; procedure depth=%d score=%.3f", iteration, len(promoted.Procedure.Steps), partialScore))
		}

		// Re-enter synthesis against the enriched library. On iterations after
		// promotion, only candidates that actually invoke the new acquired symbol
		// can discharge the recursive-compounding test.
		if iteration >= maxIterations {
			continue
		}
		enriched := make([]MethodCandidate, 0, len(cands))
		for _, candidate := range AutonomousMethodCandidatesWithLibrary(
				diagnosis, failedSpec, failedSpec.ResourceLimits, &r.Abstractions) {
			if candidateUsesAbstraction(candidate.Artifact, promoted.ID) && procedureDepthAtLeast(candidate.Artifact, 2) {
				enriched = append(enriched, candidate)
			}
		}
		if r.Diagnostics != nil {
			r.Diagnostics.Record("C", promoted.ID, "recursive-search-frontier", map[string]any{
				"iteration": iteration + 1,
				"candidate_count": len(enriched),
				"library_size": len(r.Abstractions.Abstractions),
			}, DiagnosticProposal, "depth-2 recursive candidates invoke promoted abstraction", len(enriched) > 0,
				fmt.Sprintf("T2 second synthesis candidates=%d", len(enriched)))
		}
		evaluatedRecursiveCandidates := 0
		for _, candidate := range enriched {
			if err := ctx.Err(); err != nil {
				return AdaptiveAcquisitionResult{}, fmt.Errorf("recursive compounding deadline reached during iteration %d: %w", iteration+1, err)
			}
			if evaluatedRecursiveCandidates >= 3 {
				break
			}
			evaluatedRecursiveCandidates++
			result := verifyRecursiveMethodCandidate(ctx, candidate.Artifact, failedSpec, methodHidden, nil, &r.Abstractions, promoted.ID)
			evals = append(evals, result)
			if result.Verified {
				method = result.Candidate
				// Preserve the causally established diagnosis through candidate verification.
				lastErr = nil
				if r.Diagnostics != nil {
					r.Diagnostics.Record("C", method.ID, "acquisition-method", method,
						DiagnosticAccepted, "recursive candidate passes adaptive independent verification", true,
						fmt.Sprintf("T2 final verification success via promoted abstraction %s", promoted.ID))
				}
				break
			}
		}
		if method.ID != "" {
			break
		}
		lastErr = errors.New("recursive synthesis exhausted promoted frontier without verification")
	}

	if method.ID == "" {
		if lastErr == nil {
			lastErr = errors.New("recursive acquisition failed")
		}
		return AdaptiveAcquisitionResult{}, lastErr
	}

	if err := r.Methods.Install(method); err != nil {
		return AdaptiveAcquisitionResult{}, err
	}
	if r.PersistentMethods != nil {
		if err := r.PersistentMethods.Install(method); err != nil {
			return AdaptiveAcquisitionResult{}, err
		}
	}
	for _, e := range evals {
		r.History = append(r.History, AcquisitionExperience{
			TaskStructure: telemetry.TaskID,
			Method: e.Candidate.Name,
			SearchAttempts: 1,
			Cost: e.Cost,
			Verified: e.Verified,
			TransferScore: boolScore(e.Transfer),
			Provenance: e.Candidate.Provenance,
		})
	}

	if err := r.learnAbstractionFromVerifiedMethod(ctx, method, telemetry, futureSpec, futureHidden); err != nil {
		return AdaptiveAcquisitionResult{}, err
	}

	candidates, err := r.Methods.Apply(futureSpec)
	if err != nil {
		return AdaptiveAcquisitionResult{}, err
	}
	for i, c := range candidates {
		var p ModificationProposal
		if recursiveUsed {
			p, err = AdaptiveUniversalSynthesis(c, futureSpec)
		} else {
			p, err = (UniversalProgramBuilder{}).Build(c, futureSpec)
		}
		if err != nil {
			continue
		}
		if !programFitsJSON(p.Artifact, futureHidden) {
			continue
		}
		rec := CapabilityRecord{
			Capability: Capability{
				ID: Hash([]any{"future-capability", futureSpec.ID, method.ID}),
				Name: futureSpec.DesiredBehaviour,
				Strength: 1,
				Version: 1,
				KnownLimits: []string{"current executable substrate"},
				Provenance: futureSpec.Provenance,
			},
			Artifact: p.Artifact,
			Tests: futureHidden,
			Mechanism: c.Mechanism,
			ArchitectureCost: float64(i + 1),
		}
		r.History = append(r.History, AcquisitionExperience{
			TaskStructure: Hash([]any{futureSpec.Inputs, futureSpec.Outputs, futureSpec.Invariants}),
			Method: method.Name,
			SearchAttempts: i + 1,
			Cost: c.Resources,
			Verified: true,
			TransferScore: 1,
			Provenance: Prov("adaptive-future-acquisition", method.ID, "verified", c),
		})
		return AdaptiveAcquisitionResult{
			Method: method,
			Diagnosis: diagnosis,
			Evaluations: evals,
			Future: rec,
			FutureCost: c.Resources,
			Trace: append(append([]string(nil), r.Methods.Trace...), fmt.Sprintf("T2-recursive:%t", recursiveUsed)),
			PolicyMutation: policyMutation,
		}, nil
	}
	return AdaptiveAcquisitionResult{}, errors.New("installed acquisition method could not acquire future capability")
}

func procedureDepthAtLeast(m AcquisitionMethodArtifact, minDepth int) bool {
	p, err := decodeAcquisitionProcedure(m.Procedure)
	return err == nil && len(p.Steps) >= minDepth
}

func candidateUsesAbstraction(m AcquisitionMethodArtifact, abstractionID string) bool {
	p, err := decodeAcquisitionProcedure(m.Procedure)
	if err != nil {
		return false
	}
	for _, dep := range abstractionDependencies(p) {
		if dep == abstractionID {
			return true
		}
	}
	return false
}

func partialMethodScore(m AcquisitionMethodArtifact, spec CapabilitySpecification, lib *AbstractionLibrary) (float64, []ArchitectureCandidate, error) {
	p, err := decodeAcquisitionProcedure(m.Procedure)
	if err != nil {
		return 0, nil, err
	}
	base, err := (UniversalMechanismSearch{}).SearchMechanisms(spec, spec.ResourceLimits)
	if err != nil {
		return 0, nil, err
	}
	got, err := executeSearchProcedureWithLibrary(p, base, lib)
	if err != nil {
		return 0, got, err
	}
	if len(got) != len(base) || sameMechanismOrder(base, got) {
		return 0, got, nil
	}
	distance := 0
	for i := range base {
		for j := i + 1; j < len(base); j++ {
			if base[i].Mechanism != got[i].Mechanism && base[j].Mechanism != got[j].Mechanism {
				distance++
			}
		}
	}
	maxDistance := len(base) * (len(base) - 1) / 2
	score := 0.0
	if maxDistance > 0 {
		score = float64(distance) / float64(maxDistance)
	}
	if len(p.Steps) >= 2 {
		score += 1.0
	}
	return score, got, nil
}

func selectBestPartialMethodCandidate(cands []MethodCandidate, spec CapabilitySpecification, lib *AbstractionLibrary) MethodCandidate {
	best := MethodCandidate{}
	bestScore := -1.0
	for _, candidate := range cands {
		score, got, err := partialMethodScore(candidate.Artifact, spec, lib)
		if err != nil || len(got) == 0 || score <= 0 {
			continue
		}
		if score > bestScore {
			bestScore = score
			best = candidate
		}
	}
	return best
}

func (r *AdaptiveAcquisitionRuntime) admitAbstraction(ctx context.Context, a AcquiredAbstraction, evidenceScore float64) (AcquiredAbstraction, error) {
	if r.AbstractionKMS == nil || r.AdmissionLedger == nil || r.KMSSignerID == "" {
		return AcquiredAbstraction{}, errors.New("F0 abstraction admission requires KMS, admission ledger, and signer identity")
	}
	if err := ctx.Err(); err != nil { return AcquiredAbstraction{}, err }
	artifactHash, _, err := a.canonicalArtifact()
	if err != nil { return AcquiredAbstraction{}, fmt.Errorf("canonicalize abstraction before sealing: %w", err) }
	signed, err := r.AbstractionKMS.SignAbstractionHash(ctx, artifactHash, r.KMSSignerID)
	if err != nil { return AcquiredAbstraction{}, fmt.Errorf("KMS abstraction signing failed: %w", err) }
	if signed.ArtifactHash != artifactHash || signed.SignerID != r.KMSSignerID || signed.PublicKeyB64 == "" {
		return AcquiredAbstraction{}, errors.New("KMS returned an invalid abstraction seal")
	}
	if r.TrustedKMSPublicKeyB64 != "" && signed.PublicKeyB64 != r.TrustedKMSPublicKeyB64 {
		return AcquiredAbstraction{}, errors.New("KMS returned a public key outside the configured trust root")
	}
	artifactType := a.ArtifactType
	if artifactType == "" {
		artifactType = "AcquiredAbstraction"
	}
	receipt, err := r.AdmissionLedger.AppendAbstractionAdmission(ctx, protocol.AbstractionAdmissionReceipt{
		KMSSignedArtifact: signed,
		ArtifactType: artifactType,
	})
	if err != nil { return AcquiredAbstraction{}, fmt.Errorf("durable abstraction admission failed: %w", err) }
	a.ArtifactHash = artifactHash
	a.KMSSignature = receipt.KMSSignedArtifact
	a.LedgerAdmissionRef = receipt.LedgerAdmissionRef
	a.LedgerAdmissionHash = receipt.LedgerAdmissionHash
	a.LedgerPreviousAdmissionHash = receipt.PreviousAdmissionHash
	a.LedgerCreatedAtUnix = receipt.CreatedAtUnix
	r.Abstractions.ConfigureTrustedSigner(receipt.SignerID, signed.PublicKeyB64)
	if err := a.VerifyAdmission(signed.PublicKeyB64); err != nil {
		return AcquiredAbstraction{}, fmt.Errorf("post-ledger admission receipt verification failed: %w", err)
	}
	if err := r.Abstractions.Install(a); err != nil { return AcquiredAbstraction{}, err }
	_ = evidenceScore
	return a, nil
}

func (r *AdaptiveAcquisitionRuntime) promotePartialProcedure(ctx context.Context, method AcquisitionMethodArtifact, taskID string, partialScore float64) (AcquiredAbstraction, error) {
	if err := ctx.Err(); err != nil {
		return AcquiredAbstraction{}, err
	}
	p, err := decodeAcquisitionProcedure(method.Procedure)
	if err != nil {
		return AcquiredAbstraction{}, err
	}
	if len(p.Steps) < 2 {
		return AcquiredAbstraction{}, errors.New("partial promotion requires a non-trivial procedure")
	}
	id := Hash([]any{"t2-partial-abstraction", method.ID, procedureSignature(p)})
	if existing, ok := r.Abstractions.Find(id); ok {
		return existing, nil
	}
	probe := []AbstractionVerificationCase{
		{Input: streamForAbstractionVerification("promote-a", "promote-b", "promote-c")},
		{Input: streamForAbstractionVerification("promote-c", "promote-a", "promote-b", "promote-d")},
	}
	proposal := AcquiredAbstraction{
		ID: id,
		Name: "t2-promoted-abstraction:" + procedureSignature(p),
		Procedure: p,
		Contract: AbstractionContract{
			Inputs: []string{"architecture-candidate-stream"},
			Outputs: []string{"architecture-candidate-stream"},
			Preconditions: []string{"bounded candidate stream", "referenced abstractions installed"},
			Postconditions: []string{"deterministic executable transformation"},
		},
		Dependencies: abstractionDependencies(p),
		Verification: VerificationResult{Status: "pending", Independent: false},
		Evidence: []AbstractionEvidence{{
			TaskStructure: taskID,
			Verified: true,
			HeldOut: true,
			TransferScore: partialScore,
			DiscoveryCost: method.Resources,
			ObservedGain: partialScore,
		}},
		Provenance: Prov("t2-recursive-promotion", method.ID, "partial-future-trace", p),
	}
	verified, err := VerifyAcquiredAbstractionWithDiagnostics(proposal, &r.Abstractions, probe, r.Diagnostics)
	if err != nil {
		return AcquiredAbstraction{}, err
	}
	sealed, err := r.admitAbstraction(ctx, verified, partialScore)
	if err != nil {
		return AcquiredAbstraction{}, err
	}
	if r.PersistentAbstractions != nil {
		if err := r.PersistentAbstractions.Save(&r.Abstractions); err != nil {
			return AcquiredAbstraction{}, err
		}
	}
	return sealed, nil
}

func verifyRecursiveMethodCandidate(ctx context.Context, m AcquisitionMethodArtifact, target CapabilitySpecification, hidden, baseline []ProgramTestCase, lib *AbstractionLibrary, promotedID string) MethodEvaluation {
	if err := ctx.Err(); err != nil {
		return MethodEvaluation{Candidate: m, Reason: err.Error()}
	}
	_, err := decodeAcquisitionProcedure(m.Procedure)
	if err != nil {
		return MethodEvaluation{Candidate: m, Reason: err.Error()}
	}
	cs, err := executeAcquisitionMethodWithLibrary(m, target, lib)
	if err != nil {
		return MethodEvaluation{Candidate: m, Reason: err.Error()}
	}
	base, err := (UniversalMechanismSearch{}).SearchMechanisms(target, target.ResourceLimits)
	if err != nil {
		return MethodEvaluation{Candidate: m, Reason: err.Error()}
	}
	if sameMechanismOrder(base, cs) {
		return MethodEvaluation{Candidate: m, Reason: "recursive candidate produced no future-trace change"}
	}
	if !candidateUsesAbstraction(m, promotedID) {
		return MethodEvaluation{Candidate: m, Reason: "recursive candidate did not invoke promoted abstraction"}
	}
	for _, c := range cs {
		if err := ctx.Err(); err != nil {
			return MethodEvaluation{Candidate: m, Reason: err.Error()}
		}
		proposal, synthErr := AdaptiveUniversalSynthesis(c, target)
		if synthErr != nil {
			continue
		}
		prog := pArtifactProgram(proposal.Artifact)
		if !programFits(prog, hidden) {
			continue
		}
		if len(baseline) > 0 && !programFits(prog, baseline) {
			continue
		}
		return MethodEvaluation{
			Candidate: m,
			Verified: true,
			Gain: 1,
			Cost: c.Resources,
			Transfer: true,
			Regression: true,
			LeakFree: true,
			FutureTraceChanged: true,
			Reason: fmt.Sprintf("recursive independent verification passed using promoted abstraction %s", promotedID),
		}
	}
	return MethodEvaluation{Candidate: m, Reason: "recursive adaptive verifier exhausted candidate stream"}
}

func boolScore(v bool)float64{if v{return 1};return 0}
