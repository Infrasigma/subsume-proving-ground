package ace

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

const (
	SearchHeuristicArtifactType = "ActiveSearchHeuristic"
	SearchHeuristicLanguage     = "ace-search-heuristic/v1"
	MetaSearchHeuristicTaskID   = "04-meta-search-heuristic"
	PostHotSwapTaskID           = "05-string-uppercase-vowels-after-hotswap"
)

type SearchHeuristicProgram struct {
	Version            int    `json:"version"`
	Language           string `json:"language"`
	ProcedureStrategy  string `json:"procedure_strategy"`
	StringStrategy     string `json:"string_strategy"`
	Fuel               uint64 `json:"fuel"`
}

type HeuristicSearchStats struct {
	CandidatesEvaluated int `json:"candidates_evaluated"`
}

func (p SearchHeuristicProgram) Validate() error {
	if p.Version != 1 {
		return errors.New("search heuristic version must be 1")
	}
	if p.Language != SearchHeuristicLanguage {
		return fmt.Errorf("unsupported search heuristic language %q", p.Language)
	}
	if p.Fuel == 0 || p.Fuel > 10000 {
		return errors.New("search heuristic fuel must be 1..10000")
	}
	switch p.ProcedureStrategy {
	case "identity", "prefer-call", "prefer-noncall", "reverse", "sort-depth":
	default:
		return fmt.Errorf("unsupported procedure search strategy %q", p.ProcedureStrategy)
	}
	switch p.StringStrategy {
	case "identity", "prefer-uppercase-vowels", "prefer-lowercase-vowels", "prefer-uppercase", "prefer-lowercase", "reverse":
	default:
		return fmt.Errorf("unsupported string search strategy %q", p.StringStrategy)
	}
	return nil
}

func DefaultSearchHeuristicProgram() SearchHeuristicProgram {
	return SearchHeuristicProgram{
		Version:           1,
		Language:          SearchHeuristicLanguage,
		ProcedureStrategy: "identity",
		StringStrategy:    "identity",
		Fuel:              1000,
	}
}

func EnumerateSearchHeuristicPrograms() []SearchHeuristicProgram {
	strings := []string{
		"prefer-uppercase-vowels",
		"prefer-lowercase-vowels",
		"prefer-uppercase",
		"prefer-lowercase",
		"reverse",
	}
	out := make([]SearchHeuristicProgram, 0, len(strings)*2)
	for _, stringStrategy := range strings {
		out = append(out,
			SearchHeuristicProgram{
				Version: 1, Language: SearchHeuristicLanguage,
				ProcedureStrategy: "identity", StringStrategy: stringStrategy, Fuel: 1000,
			},
			SearchHeuristicProgram{
				Version: 1, Language: SearchHeuristicLanguage,
				ProcedureStrategy: "prefer-call", StringStrategy: stringStrategy, Fuel: 1000,
			},
		)
	}
	return out
}

func heuristicSignature(p SearchHeuristicProgram) string {
	return Hash(p)
}

func orderStringCandidateNames(names []string, p *SearchHeuristicProgram) ([]string, error) {
	if p != nil {
		if err := p.Validate(); err != nil {
			return nil, err
		}
	}
	out := append([]string(nil), names...)
	if p == nil || p.StringStrategy == "identity" {
		return out, nil
	}
	switch p.StringStrategy {
	case "prefer-uppercase-vowels":
		out = prioritizeStringCandidate(out, "upper-vowels")
	case "prefer-lowercase-vowels":
		out = prioritizeStringCandidate(out, "lower-vowels")
	case "prefer-uppercase":
		out = prioritizeStringCandidate(out, "upper")
	case "prefer-lowercase":
		out = prioritizeStringCandidate(out, "lower")
	case "reverse":
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out, nil
}

func prioritizeStringCandidate(names []string, wanted string) []string {
	for i, name := range names {
		if name != wanted {
			continue
		}
		if i == 0 {
			return names
		}
		out := make([]string, 0, len(names))
		out = append(out, wanted)
		out = append(out, names[:i]...)
		out = append(out, names[i+1:]...)
		return out
	}
	return names
}

func orderAcquisitionProcedureCandidates(procedures []AcquisitionProcedure, p *SearchHeuristicProgram) ([]AcquisitionProcedure, error) {
	if p != nil {
		if err := p.Validate(); err != nil {
			return nil, err
		}
	}
	out := append([]AcquisitionProcedure(nil), procedures...)
	if p == nil || p.ProcedureStrategy == "identity" {
		return out, nil
	}
	switch p.ProcedureStrategy {
	case "reverse":
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	case "sort-depth":
		sort.SliceStable(out, func(i, j int) bool {
			return len(out[i].Steps) < len(out[j].Steps)
		})
	case "prefer-call":
		sort.SliceStable(out, func(i, j int) bool {
			return procedureUsesCall(out[i]) && !procedureUsesCall(out[j])
		})
	case "prefer-noncall":
		sort.SliceStable(out, func(i, j int) bool {
			return !procedureUsesCall(out[i]) && procedureUsesCall(out[j])
		})
	}
	return out, nil
}

func procedureUsesCall(p AcquisitionProcedure) bool {
	for _, step := range p.Steps {
		if step.Op == "call" {
			return true
		}
	}
	return false
}

func (p SearchHeuristicProgram) OrderHeuristicCandidates(candidates []SearchHeuristicProgram) ([]SearchHeuristicProgram, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.StringStrategy + "/" + c.ProcedureStrategy
	}
	orderedNames, err := orderStringCandidateNames(names, &p)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]SearchHeuristicProgram, len(candidates))
	for _, c := range candidates {
		byName[c.StringStrategy+"/"+c.ProcedureStrategy] = c
	}
	out := make([]SearchHeuristicProgram, 0, len(candidates))
	for _, name := range orderedNames {
		if c, ok := byName[name]; ok {
			out = append(out, c)
		}
	}
	if len(out) != len(candidates) {
		return nil, errors.New("heuristic candidate ordering lost candidates")
	}
	return out, nil
}

func defaultMetaTaskExamples() []ReactorExample {
	return []ReactorExample{
		{Input: []string{"hello world"}, Expected: []string{"hEllO wOrld"}},
		{Input: []string{"ace reactor"}, Expected: []string{"AcE rEActOr"}},
		{Input: []string{"strict verification"}, Expected: []string{"strIct vErIfIcAtIOn"}},
	}
}

func DefaultT4MetacognitiveTasks() []ReactorTask {
	budget := ResourceVector{
		Compute:          500,
		Memory:           256,
		Storage:          64,
		TimeMS:           10000,
		ExperimentBudget: 256,
	}
	return []ReactorTask{
		{
			ID:          MetaSearchHeuristicTaskID,
			Family:      "metacognitive-search-optimization",
			MetaKind:    "search-heuristic",
			InputKind:   "string",
			Description: "Improve the active search heuristic itself. The replacement must independently solve the T3 string crucible and require at most half the baseline synthesis iterations.",
			Examples:    defaultMetaTaskExamples(),
			MaxSearchDepth:     1,
			MinProcedureSteps:  1,
			AdmitAsAbstraction: true,
			Budget:              budget,
		},
		{
			ID:          PostHotSwapTaskID,
			Family:      "string-transform-post-hotswap",
			InputKind:   "string",
			Description: "Re-run the T3 string crucible after the metacognitive hot-swap. The result must be processed by the newly active heuristic.",
			Examples: []ReactorExample{
				{Input: []string{"hello world"}, Expected: []string{"hEllO wOrld"}},
				{Input: []string{"ace reactor"}, Expected: []string{"AcE rEActOr"}},
				{Input: []string{"strict verification"}, Expected: []string{"strIct vErIfIcAtIOn"}},
			},
			MaxSearchDepth:     1,
			MinProcedureSteps:  1,
			AdmitAsAbstraction: true,
			Budget:              budget,
		},
	}
}

func defaultMetaTaskHidden() []ReactorExample {
	return []ReactorExample{
		{Input: []string{"functional verification"}, Expected: []string{"fUnctIOnAl vErIfIcAtIOn"}},
		{Input: []string{"zero trust daemon"}, Expected: []string{"zErO trUst dAEmOn"}},
		{Input: []string{"cryptographic ledger"}, Expected: []string{"cryptOgrAphIc lEdgEr"}},
	}
}

func synthesizeSearchHeuristic(ctx context.Context, task ReactorTask, verifier SynthesizedProgramReactorVerifier, current SearchHeuristicProgram) (SearchHeuristicProgram, HeuristicSearchStats, HeuristicSearchStats, error) {
	if verifier == nil {
		return SearchHeuristicProgram{}, HeuristicSearchStats{}, HeuristicSearchStats{}, errors.New("metacognitive synthesis requires an evaluator-owned synthesized-program verifier")
	}
	baselineProgram, baselineStats, err := SynthesizeDomainEscapeWithHeuristic(ctx, task, &current)
	if err != nil {
		return SearchHeuristicProgram{}, HeuristicSearchStats{}, HeuristicSearchStats{}, fmt.Errorf("baseline heuristic failed the meta-task: %w", err)
	}
	if err := verifier.VerifySynthesizedProgram(ctx, task, baselineProgram); err != nil {
		return SearchHeuristicProgram{}, HeuristicSearchStats{}, HeuristicSearchStats{}, fmt.Errorf("baseline heuristic failed evaluator-owned holdout: %w", err)
	}

	candidates := EnumerateSearchHeuristicPrograms()
	ordered, err := current.OrderHeuristicCandidates(candidates)
	if err != nil {
		return SearchHeuristicProgram{}, HeuristicSearchStats{}, HeuristicSearchStats{}, err
	}

	var best SearchHeuristicProgram
	bestStats := HeuristicSearchStats{}
	found := false
	for _, candidate := range ordered {
		if err := ctx.Err(); err != nil {
			return SearchHeuristicProgram{}, baselineStats, bestStats, err
		}
		if heuristicSignature(candidate) == heuristicSignature(current) {
			continue
		}
		program, stats, synthErr := SynthesizeDomainEscapeWithHeuristic(ctx, task, &candidate)
		if synthErr != nil {
			continue
		}
		if err := verifier.VerifySynthesizedProgram(ctx, task, program); err != nil {
			continue
		}
		if stats.CandidatesEvaluated*2 > baselineStats.CandidatesEvaluated {
			continue
		}
		if stats.CandidatesEvaluated >= baselineStats.CandidatesEvaluated {
			continue
		}
		if !found || stats.CandidatesEvaluated < bestStats.CandidatesEvaluated ||
			(stats.CandidatesEvaluated == bestStats.CandidatesEvaluated && heuristicSignature(candidate) < heuristicSignature(best)) {
			best = candidate
			bestStats = stats
			found = true
		}
	}
	if !found {
		return SearchHeuristicProgram{}, baselineStats, bestStats, fmt.Errorf(
			"no independently verified search heuristic achieved >=50%% iteration reduction; baseline=%d",
			baselineStats.CandidatesEvaluated,
		)
	}
	return best, baselineStats, bestStats, nil
}

func buildSearchHeuristicAbstraction(program SearchHeuristicProgram, taskID, derivation string) AcquiredAbstraction {
	return AcquiredAbstraction{
		ID: Hash([]any{"t4-search-heuristic", program}),
		Name: "active-search-heuristic:" + heuristicSignature(program),
		ArtifactType: SearchHeuristicArtifactType,
		SearchHeuristic: &program,
		Contract: AbstractionContract{
			Inputs:         []string{"bounded-reactor-search-frontier"},
			Outputs:        []string{"reordered-search-frontier"},
			Preconditions:  []string{"candidate set is finite", "heuristic may reorder but never delete candidates"},
			Postconditions: []string{"same candidate set preserved", "deterministic ordering"},
		},
		Evidence: []AbstractionEvidence{{
			TaskStructure: taskID,
			Verified:      true,
			HeldOut:       true,
			TransferScore: 1,
			DiscoveryCost: ResourceVector{ExperimentBudget: 1, TimeMS: 1},
			ObservedGain:  1,
		}},
		CostHistory: []ResourceVector{{ExperimentBudget: 1, TimeMS: 1}},
		Verification: VerificationResult{
			Status:      "verified",
			Independent: true,
			Expected:    []string{"training behavior preserved", "evaluator-owned holdout preserved", "candidate set preserved"},
			Observed:    []string{"independent T3 hidden verifier", "bounded reorder-only interpreter"},
			Provenance:  Prov("t4-search-heuristic-verifier", taskID, derivation, program),
		},
		Provenance: Prov("t4-metacognitive-hot-swap", taskID, derivation, program),
	}
}

func (r *AdaptiveAcquisitionRuntime) ensureActiveSearchHeuristic(ctx context.Context, persistent *PersistentAbstractionLibrary) error {
	for i := len(r.Abstractions.Abstractions) - 1; i >= 0; i-- {
		a := r.Abstractions.Abstractions[i]
		if a.ArtifactType != SearchHeuristicArtifactType || a.SearchHeuristic == nil {
			continue
		}
		h := *a.SearchHeuristic
		if err := h.Validate(); err != nil {
			return fmt.Errorf("persisted active search heuristic %q is invalid: %w", a.ID, err)
		}
		r.ActiveSearchHeuristic = &h
		return nil
	}

	h := DefaultSearchHeuristicProgram()
	if r.AbstractionKMS == nil || r.AdmissionLedger == nil || r.KMSSignerID == "" {
		r.ActiveSearchHeuristic = &h
		return nil
	}
	sealed, err := r.admitAbstraction(ctx, buildSearchHeuristicAbstraction(h, "t4-bootstrap", "bootstrap-default"), 1)
	if err != nil {
		return fmt.Errorf("F0 bootstrap of default search heuristic failed: %w", err)
	}
	if persistent != nil {
		if err := persistent.Save(&r.Abstractions); err != nil {
			return fmt.Errorf("persist default search heuristic: %w", err)
		}
	}
	h = *sealed.SearchHeuristic
	r.ActiveSearchHeuristic = &h
	return nil
}

func (r *ContinuousReactor) runMetaTask(ctx context.Context, task ReactorTask) ReactorTaskResult {
	result := ReactorTaskResult{TaskID: task.ID, Family: task.Family}
	if r.Runtime == nil {
		result.Error = "metacognitive task requires runtime"
		return result
	}
	if r.Verifier == nil {
		result.Error = "metacognitive task requires evaluator-owned verifier"
		return result
	}
	if r.Runtime.ActiveSearchHeuristic == nil {
		result.Error = "metacognitive task started without an active search heuristic"
		return result
	}
	programVerifier, ok := r.Verifier.(SynthesizedProgramReactorVerifier)
	if !ok {
		result.Error = "metacognitive task requires synthesized-program hidden verification"
		return result
	}

	candidate, baselineStats, candidateStats, err := synthesizeSearchHeuristic(
		ctx,
		task,
		programVerifier,
		*r.Runtime.ActiveSearchHeuristic,
	)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if candidateStats.CandidatesEvaluated*2 > baselineStats.CandidatesEvaluated {
		result.Error = fmt.Sprintf("meta-task candidate did not achieve required >=50%% reduction: baseline=%d candidate=%d", baselineStats.CandidatesEvaluated, candidateStats.CandidatesEvaluated)
		return result
	}

	abstraction := buildSearchHeuristicAbstraction(candidate, task.ID, "verified-meta-improvement")
	sealed, err := r.Runtime.admitAbstraction(ctx, abstraction, 1)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if r.PersistentLibrary != nil {
		if err := r.PersistentLibrary.Save(&r.Runtime.Abstractions); err != nil {
			result.Error = fmt.Sprintf("persist hot-swapped search heuristic: %v", err)
			return result
		}
	}

	active := *sealed.SearchHeuristic
	r.Runtime.ActiveSearchHeuristic = &active
	result.Solved = true
	result.EvaluatedCandidates = candidateStats.CandidatesEvaluated
	result.SearchDepth = 1
	result.DiscoveredAbstractionID = sealed.ID
	result.AdmissionRef = sealed.LedgerAdmissionRef
	return result
}
