package acex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Infrasigma/subsume-proving-ground/internal/ace"
	"sort"
)

type V8CapabilityEvidence struct {
	Capability string
	EvidenceID string
	Verified   bool
	Details    string
}

type V8CognitiveEntity struct {
	Mechanism        V5MechanismRuntime
	StaticLibrary    V4Library
	ActiveStrategy   V6Strategy
	StrategyHistory  []V6Strategy
	Experience       *V7CognitiveAgent
	Memory           V5AdaptiveMemory
	StructuralRoles  V8StructuralRoleLearner
	AdaptiveRoles    V8AdaptiveStructuralRoleLearner
	DirectedRepresentation V11DirectedExecutableRepresentation
	RelationalPatterns V8RelationalPatternInducer
	ExecutableRepresentation V8ExecutableRepresentation
	Inquiry          InquiryManager
	PendingIntervention string
	Evidence         []V8CapabilityEvidence
	HermeticArtifact []byte
	HermeticArtifactHash string
	HermeticExecutor *V12WasmDecisionExecutor
	HermeticScratchCandidates [V12DecisionMaxCandidates]V12CandidateMatrix
	HermeticScratchInput [V12DecisionMaxCandidates * V12DecisionCandidateStride]byte
	Version          uint64
	Failures         []string
}

func NewV8CognitiveEntity() *V8CognitiveEntity {
	return &V8CognitiveEntity{
		Mechanism:     *NewV5MechanismRuntime(),
		StaticLibrary: NewV4Library(),
		ActiveStrategy: V6Strategy{Name:"baseline", Strategy:V6BaselineSearch, Library:NewV4Library()},
		Experience:    NewV7CognitiveAgent(),
		StructuralRoles: NewV8StructuralRoleLearner(),
		AdaptiveRoles: NewV8AdaptiveStructuralRoleLearner(),
		DirectedRepresentation: NewV11DirectedExecutableRepresentation(),
		RelationalPatterns: NewV8RelationalPatternInducer(),
		ExecutableRepresentation: NewV8ExecutableRepresentation(),
	}
}

func (e *V8CognitiveEntity) attest(capability, evidenceID, details string, verified bool) {
	e.Evidence = append(e.Evidence, V8CapabilityEvidence{
		Capability: capability,
		EvidenceID: evidenceID,
		Verified: verified,
		Details: details,
	})
}

func (e *V8CognitiveEntity) Remember(trace V5MemoryTrace) error {
	if e == nil {
		return errors.New("nil V8 entity")
	}
	if err := e.Memory.Record(trace); err != nil {
		return err
	}
	e.attest("persistent-surprise-memory", trace.ID, "recorded verified experience trace", trace.Verified)
	return nil
}

func (e *V8CognitiveEntity) Retrieve(context []string, limit int) []V5MemoryTrace {
	if e == nil {
		return nil
	}
	items := e.Memory.Retrieve(context, limit)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].PredictionErr != items[j].PredictionErr {
			return items[i].PredictionErr > items[j].PredictionErr
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func (e *V8CognitiveEntity) Inquire(h []V5Hypothesis, observedAction, observedOutcome string) error {
	if e == nil {
		return errors.New("nil V8 entity")
	}
	plan, err := V5ChooseIntervention(h)
	if err != nil {
		return err
	}
	if observedAction == "" {
		e.PendingIntervention = plan.Action
		e.attest("causal-inquiry", "v8-inquiry-"+plan.Action, "selected discriminating intervention", true)
		return nil
	}
	post, err := V5ReviseHypotheses(h, observedAction, observedOutcome)
	if err != nil {
		e.Failures = append(e.Failures, "causal-model-refuted:"+observedAction)
		return err
	}
	if e.PendingIntervention == observedAction { e.PendingIntervention = "" }
	e.attest("causal-inquiry", "v8-belief-"+observedAction, fmt.Sprintf("surviving-hypotheses=%d", len(post)), true)
	return nil
}

func (e *V8CognitiveEntity) ObserveAndAct(state RelationalState, actions []string) (string, error) {
	if e == nil || e.Experience == nil {
		return "", errors.New("V8 experience core unavailable")
	}
	stateKey := V7StateKey(state)
	if e.PendingIntervention != "" {
		for _, action := range actions {
			if action == e.PendingIntervention {
				e.attest("causal-action-selection", "v8-intervention-"+action, "pending discriminating intervention selected", true)
				return action, nil
			}
		}
	}
	blocked := map[string]bool{}
	for _, memory := range e.Memory.Retrieve([]string{stateKey}, 16) {
		if !memory.Failure || memory.PredictionErr < 0.5 {
			continue
		}
		for _, token := range memory.Context {
			for _, action := range actions {
				if token == action {
					blocked[action] = true
				}
			}
		}
	}
	filtered := make([]string, 0, len(actions))
	for _, action := range actions {
		if !blocked[action] {
			filtered = append(filtered, action)
		}
	}
	if len(filtered) == 0 {
		filtered = append(filtered, actions...)
	}
	if action, ok := e.StructuralRoles.Select(state, filtered); ok {
		e.attest("structural-role-transfer", "v8-role-"+V7StateKey(state),
			"selected previously verified label-invariant structural action role", true)
		return action, nil
	}
	if action, ok := e.AdaptiveRoles.Select(state, filtered); ok {
		e.attest("adaptive-representation-transfer", "v8-adaptive-"+V7StateKey(state),
			e.AdaptiveRoles.InventedRepresentation(), true)
		return action, nil
	}
	// Once a directed representation is retained, its action decision is owned
	// exclusively by the content-addressed Wasm artifact. There is deliberately
	// no Go matcher fallback on this path.
	if e.DirectedRepresentation.Retained {
		action, err := e.decideHermetic(state, filtered)
		if err != nil {
			return "", err
		}
		e.attest("directed-executable-representation-transfer", "v12-wasm-"+e.HermeticArtifactHash,
			fmt.Sprintf("complexity=%d expansions=%d retained=true wasm=%s", e.DirectedRepresentation.Complexity(), e.DirectedRepresentation.SearchExpansions, e.HermeticArtifactHash), true)
		return action, nil
	}

	// Before retention, evidence is still being gathered. The provisional Go
	// matcher is permitted only in this pre-retention phase; after retention
	// this branch is unreachable and all decisions are forced through Wasm.
	if action, ok := e.DirectedRepresentation.Select(state, filtered); ok {
		e.attest("directed-executable-representation-provisional", "v11-provisional-"+e.DirectedRepresentation.Key(),
			fmt.Sprintf("complexity=%d expansions=%d retained=false examples=%d", e.DirectedRepresentation.Complexity(), e.DirectedRepresentation.SearchExpansions, len(e.DirectedRepresentation.Examples)), true)
		return action, nil
	}

	if e.DirectedRepresentation.Synthesize() {
		if e.DirectedRepresentation.Retained {
			action, err := e.decideHermetic(state, filtered)
			if err != nil {
				return "", err
			}
			e.Version++
			e.attest("directed-executable-representation-invention", "v12-wasm-"+e.HermeticArtifactHash,
				fmt.Sprintf("complexity=%d expansions=%d retained=true wasm=%s", e.DirectedRepresentation.Complexity(), e.DirectedRepresentation.SearchExpansions, e.HermeticArtifactHash), true)
			return action, nil
		}
		if action, ok := e.DirectedRepresentation.Select(state, filtered); ok {
			e.Version++
			e.attest("directed-executable-representation-invention-provisional", "v11-provisional-"+e.DirectedRepresentation.Key(),
				fmt.Sprintf("complexity=%d expansions=%d retained=false examples=%d", e.DirectedRepresentation.Complexity(), e.DirectedRepresentation.SearchExpansions, len(e.DirectedRepresentation.Examples)), true)
			return action, nil
		}
	}
	if action, ok := e.RelationalPatterns.Select(state, filtered); ok {
		e.attest("synthesized-relational-representation", "v8-pattern-"+e.RelationalPatterns.Pattern.Key(),
			fmt.Sprintf("complexity=%d expansions=%d", e.RelationalPatterns.Pattern.Complexity(), e.RelationalPatterns.SearchExpansions), true)
		return action, nil
	}
	if e.RelationalPatterns.TrySynthesize() {
		if action, ok := e.RelationalPatterns.Select(state, filtered); ok {
			e.Version++
			e.attest("synthesized-relational-representation", "v8-pattern-"+e.RelationalPatterns.Pattern.Key(),
				fmt.Sprintf("complexity=%d expansions=%d", e.RelationalPatterns.Pattern.Complexity(), e.RelationalPatterns.SearchExpansions), true)
			return action, nil
		}
	}
	if action, ok := e.ExecutableRepresentation.Select(state, filtered); ok {
		e.attest("executable-representation-transfer", "v8-executable-rep-"+e.ExecutableRepresentation.ProgramKey,
			e.ExecutableRepresentation.Description(), true)
		return action, nil
	}
	if e.ExecutableRepresentation.Synthesize() {
		if action, ok := e.ExecutableRepresentation.Select(state, filtered); ok {
			e.Version++
			e.attest("executable-representation-invention", "v8-executable-rep-"+e.ExecutableRepresentation.ProgramKey,
				e.ExecutableRepresentation.Description(), true)
			return action, nil
		}
	}
	action, err := e.Experience.NextAction(state, filtered)
	if err != nil {
		e.Failures = append(e.Failures, "action-selection:"+err.Error())
		return "", err
	}
	e.attest("interactive-action-selection", "v8-act-"+stateKey,
		fmt.Sprintf("selected action from verified experience/novelty; blocked=%v", sortedStringSet(blocked)), true)
	return action, nil
}

func sortedStringSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		if v { out = append(out, k) }
	}
	sort.Strings(out)
	return out
}

func (e *V8CognitiveEntity) ObserveOutcome(before RelationalState, action string, after RelationalState, reward float64, terminal bool) V7Step {
	e.StructuralRoles.Observe(before, action, reward, terminal)
	e.AdaptiveRoles.Observe(before, action, reward, terminal)
	oldKey := e.DirectedRepresentation.Key()
	oldValid := e.DirectedRepresentation.Valid
	e.DirectedRepresentation.Record(before, action, reward, terminal)
	if e.DirectedRepresentation.Key() != oldKey || e.DirectedRepresentation.Valid != oldValid {
		e.closeHermeticExecutor()
		e.HermeticArtifact = nil
		e.HermeticArtifactHash = ""
	}
	e.RelationalPatterns.Record(before, action, reward, terminal)
	e.ExecutableRepresentation.Record(before, action, reward, terminal)
	step := e.Experience.ExecuteObserved(before, action, after, reward, terminal)
	err := e.Remember(V5MemoryTrace{
		ID:            "experience-" + V7ActionEffectSignature(step),
		Context:       []string{step.Before, action, step.After},
		PredictionErr: 0,
		Failure:       reward < 0,
		Utility:       reward,
		Verified:      true,
	})
	if err != nil {
		e.Failures = append(e.Failures, "memory:"+err.Error())
	}
	e.attest("verified-interactive-experience", V7ActionEffectSignature(step),
		fmt.Sprintf("reward=%.3f terminal=%t", reward, terminal), true)
	return step
}

func (e *V8CognitiveEntity) ConsolidateInteractive(traces [][]V7Step) error {
	if e == nil || e.Experience == nil {
		return errors.New("V8 experience core unavailable")
	}
	e.Experience.Consolidate(traces)
	if len(e.Experience.Macros) == 0 {
		return errors.New("no verified reusable procedure discovered")
	}
	best := e.Experience.Macros[0]
	e.attest("procedural-consolidation", best.ID,
		fmt.Sprintf("uses=%d length=%d", best.Uses, len(best.Actions)), true)
	return nil
}

func (e *V8CognitiveEntity) LearnStatic(tasks, future []V4Task, maxSize, beam int) (V6LearnResult, error) {
	if e == nil {
		return V6LearnResult{}, errors.New("nil V8 entity")
	}
	result, err := V6LearnProspective(tasks, future, e.StaticLibrary, maxSize, beam)
	if err != nil {
		e.Failures = append(e.Failures, "static-learning:"+err.Error())
		return V6LearnResult{}, err
	}
	e.StaticLibrary = result.Library
	e.attest("semantic-prospective-abstraction",
		e.StaticLibrary.Digest(),
		fmt.Sprintf("future-cost=%d->%d ratios=%v", result.FutureBefore, result.FutureAfter, result.FutureRatios), true)
	return result, nil
}

func (e *V8CognitiveEntity) SelectMechanism(visible, hidden []V4Task, maxSize, beam int) error {
	if e == nil {
		return errors.New("nil V8 entity")
	}
	strategy, ratios, err := V6SelectStrategy(
		V6Strategy{Name:"baseline", Strategy:V6BaselineSearch, Library:NewV4Library()},
		e.StaticLibrary, visible, hidden, maxSize, beam,
	)
	if err != nil {
		e.Failures = append(e.Failures, "mechanism-selection:"+err.Error())
		return err
	}
	if strategy.Strategy != V6SemanticSearch {
		return errors.New("semantic strategy was not independently selected")
	}
	e.StrategyHistory = append(e.StrategyHistory, e.ActiveStrategy)
	e.ActiveStrategy = strategy
	e.attest("mechanism-selection", "v8-semantic-strategy",
		fmt.Sprintf("hidden-ratios=%v", ratios), true)
	e.Version++
	return nil
}

func (e *V8CognitiveEntity) SolveStatic(task V4Task, maxSize, beam int) (V4SearchResult, error) {
	if e == nil {
		return V4SearchResult{}, errors.New("nil V8 entity")
	}
	if e.ActiveStrategy.Strategy == "" {
		e.ActiveStrategy = V6Strategy{Name:"baseline", Strategy:V6BaselineSearch, Library:NewV4Library()}
	}
	return V6SolveWithStrategy(e.ActiveStrategy, task, maxSize, beam)
}

func (e *V8CognitiveEntity) InventTool(task ace.Task, training, holdout []ace.ProgramTestCase) (ace.ModificationProposal, error) {
	if e == nil {
		return ace.ModificationProposal{}, errors.New("nil V8 entity")
	}
	if len(holdout) < 2 {
		return ace.ModificationProposal{}, errors.New("independent tool holdout required")
	}
	spec, err := ace.GeneralCapabilitySpecification(task, training)
	if err != nil {
		return ace.ModificationProposal{}, err
	}
	candidates, err := (ace.UniversalMechanismSearch{}).SearchMechanisms(spec, task.Budget)
	if err != nil || len(candidates) == 0 {
		return ace.ModificationProposal{}, errors.New("no executable symbolic synthesis mechanism")
	}
	for _, candidate := range candidates {
		proposal, buildErr := (ace.UniversalProgramBuilder{}).Build(candidate, spec)
		if buildErr != nil {
			continue
		}
		var program ace.UniversalProgram
		if err := json.Unmarshal([]byte(proposal.Artifact), &program); err != nil {
			continue
		}
		if !ace.ProgramFitsForTests(program, holdout) {
			continue
		}
		e.attest("tool-invention", proposal.ID,
			"symbolically synthesized executable capability; independently verified on holdout", true)
		return proposal, nil
	}
	e.Failures = append(e.Failures, "tool-invention:holdout-rejected")
	return ace.ModificationProposal{}, errors.New("tool synthesis failed independent holdout")
}

func (e *V8CognitiveEntity) RollbackMechanism() error {
	if e == nil {
		return errors.New("nil V8 entity")
	}
	if len(e.StrategyHistory) == 0 {
		return errors.New("no V8 strategy rollback")
	}
	e.ActiveStrategy = e.StrategyHistory[len(e.StrategyHistory)-1]
	e.StrategyHistory = e.StrategyHistory[:len(e.StrategyHistory)-1]
	if e.Version > 0 {
		e.Version--
	}
	e.attest("rollback", fmt.Sprintf("v8-version-%d", e.Version),
		"previous executable search strategy restored", true)
	return nil
}

func (e V8CognitiveEntity) VerifiedCapabilities() []string {
	set := map[string]bool{}
	for _, x := range e.Evidence {
		if x.Verified {
			set[x.Capability] = true
		}
	}
	out := make([]string, 0, len(set))
	for x := range set {
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}

func (e V8CognitiveEntity) HasEvidence(capability string) bool {
	for _, x := range e.Evidence {
		if x.Capability == capability && x.Verified {
			return true
		}
	}
	return false
}


func (e *V8CognitiveEntity) ensureHermeticCapability() error {
	if e == nil {
		return errors.New("nil V8 entity")
	}
	if len(e.HermeticArtifact) > 0 {
		if e.HermeticExecutor == nil {
			executor, err := NewV12WasmDecisionExecutor(context.Background(), e.HermeticArtifact)
			if err != nil {
				return err
			}
			e.HermeticExecutor = executor
		}
		return nil
	}
	ir, err := BuildV12EffectIR(e.DirectedRepresentation)
	if err != nil {
		return err
	}
	capability, err := CompileV12WasmCapability(ir)
	if err != nil {
		return err
	}
	executor, err := NewV12WasmDecisionExecutor(context.Background(), capability.Module)
	if err != nil {
		return err
	}
	e.closeHermeticExecutor()
	e.HermeticArtifact = append([]byte(nil), capability.Module...)
	e.HermeticArtifactHash = capability.SHA256
	e.HermeticExecutor = executor
	return nil
}

func (e *V8CognitiveEntity) closeHermeticExecutor() {
	if e == nil || e.HermeticExecutor == nil {
		return
	}
	_ = e.HermeticExecutor.Close(context.Background())
	e.HermeticExecutor = nil
}

func (e *V8CognitiveEntity) decideHermetic(state RelationalState, actions []string) (string, error) {
	if e == nil {
		return "", errors.New("nil V8 entity")
	}
	if !e.DirectedRepresentation.Retained {
		return "", errors.New("hermetic decision requested before retention")
	}
	if len(actions) == 0 || len(actions) > V12DecisionMaxCandidates {
		return "", fmt.Errorf("hermetic action count out of bounds: %d", len(actions))
	}
	if err := e.ensureHermeticCapability(); err != nil {
		return "", err
	}
	count, err := BuildV12RelationalCandidates(state, actions, &e.HermeticScratchCandidates)
	if err != nil {
		return "", err
	}
	input, err := EncodeV12DecisionInput(e.HermeticScratchCandidates[:count], e.HermeticScratchInput[:])
	if err != nil {
		return "", err
	}
	index, err := e.HermeticExecutor.Decide(input, uint32(count))
	if err != nil {
		return "", err
	}
	if index < 0 || int(index) >= len(actions) {
		return "", fmt.Errorf("hermetic artifact selected no valid action: %d", index)
	}
	return actions[int(index)], nil
}

func (e *V8CognitiveEntity) verifyHermeticCapability() error {
	if len(e.HermeticArtifact) == 0 {
		return errors.New("no hermetic capability artifact")
	}
	capability, err := LoadV12WasmCapability(e.HermeticArtifact)
	if err != nil {
		return err
	}
	if capability.SHA256 != e.HermeticArtifactHash {
		return errors.New("hermetic artifact hash changed")
	}
	loaded := V11DirectedExecutableRepresentation{
		Root: capability.Pattern.Root,
		Nodes: capability.Pattern.Nodes,
		Edges: append([]V11DirectedPatternEdge(nil), capability.Pattern.Edges...),
		Valid: true,
		Retained: true,
	}
	if loaded.Key() != e.DirectedRepresentation.Key() {
		return errors.New("wasm payload pattern differs from live representation")
	}
	ir, err := BuildV12EffectIR(loaded)
	if err != nil {
		return err
	}
	_, _, err = V12PatternPayload(capability.Pattern, ir)
	if err != nil {
		return err
	}
	executor, err := NewV12WasmDecisionExecutor(context.Background(), e.HermeticArtifact)
	if err != nil {
		return err
	}
	defer executor.Close(context.Background())
	input := make([]byte, V12DecisionCandidateStride)
	candidate := v12PatternCandidateMatrix(capability.Pattern)
	if _, err := EncodeV12DecisionInput([]V12CandidateMatrix{candidate}, input); err != nil {
		return err
	}
	got, err := executor.Decide(input, 1)
	if err != nil {
		return err
	}
	if got != 0 {
		return fmt.Errorf("hermetic wasm self-check returned %d", got)
	}
	return nil
}

func (e *V8CognitiveEntity) ForgetRawExperiences() {
	if e == nil {
		return
	}
	// Remove episodic traces and bounded learned mappings while preserving the
	// independently synthesized executable representation.
	e.StructuralRoles = NewV8StructuralRoleLearner()
	e.AdaptiveRoles = NewV8AdaptiveStructuralRoleLearner()
	e.RelationalPatterns = NewV8RelationalPatternInducer()
	e.Experience = NewV7CognitiveAgent()
	e.Memory = V5AdaptiveMemory{}
	e.Failures = nil
	e.Inquiry = InquiryManager{}
	e.PendingIntervention = ""
	e.Version++
	if !e.DirectedRepresentation.Retained && !e.DirectedRepresentation.Valid {
		_ = e.DirectedRepresentation.Synthesize()
	}
	if e.DirectedRepresentation.Valid {
		if err := e.DirectedRepresentation.ForgetExamples(); err != nil {
			e.Failures = append(e.Failures, "directed-retention:"+err.Error())
		}
		if e.DirectedRepresentation.Retained {
			if err := e.ensureHermeticCapability(); err != nil {
				e.Failures = append(e.Failures, "hermetic-retention:"+err.Error())
			}
		}
	}
	e.ExecutableRepresentation = e.ExecutableRepresentation.ForgetExamples()
}

func (e *V8CognitiveEntity) ExportRetainedRepresentation() (string, error) {
	if e == nil {
		return "", errors.New("nil V8 entity")
	}
	if !e.DirectedRepresentation.Retained {
		return "", errors.New("directed representation is not retained")
	}
	if err := e.ensureHermeticCapability(); err != nil {
		return "", err
	}
	if err := e.verifyHermeticCapability(); err != nil {
		return "", err
	}
	return EncodeV12WasmBase64(e.HermeticArtifact), nil
}

func (e *V8CognitiveEntity) LoadRetainedRepresentation(artifact string) error {
	if e == nil {
		return errors.New("nil V8 entity")
	}
	module, err := DecodeV12WasmBase64(artifact)
	if err != nil {
		return fmt.Errorf("V12 hermetic artifact decode failed: %w", err)
	}
	capability, err := LoadV12WasmCapability(module)
	if err != nil {
		return fmt.Errorf("V12 hermetic artifact validation failed: %w", err)
	}
	executor, err := NewV12WasmDecisionExecutor(context.Background(), module)
	if err != nil {
		return err
	}
	e.closeHermeticExecutor()
	e.DirectedRepresentation = V11DirectedExecutableRepresentation{
		Root: capability.Pattern.Root,
		Nodes: capability.Pattern.Nodes,
		Edges: append([]V11DirectedPatternEdge(nil), capability.Pattern.Edges...),
		Budget: 20000,
		Valid: true,
		Retained: true,
	}
	e.HermeticArtifact = append([]byte(nil), module...)
	e.HermeticArtifactHash = capability.SHA256
	e.HermeticExecutor = executor
	e.StructuralRoles = NewV8StructuralRoleLearner()
	e.AdaptiveRoles = NewV8AdaptiveStructuralRoleLearner()
	e.RelationalPatterns = NewV8RelationalPatternInducer()
	e.Experience = NewV7CognitiveAgent()
	e.Memory = V5AdaptiveMemory{}
	e.Failures = nil
	e.PendingIntervention = ""
	e.Version++
	return nil
}
