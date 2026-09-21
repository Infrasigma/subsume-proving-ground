package acex

import (
	"errors"
	"fmt"
	"sort"
)

type V8CapabilityEvidence struct {
	Capability string
	EvidenceID string
	Verified   bool
	Details    string
}

type V8CognitiveEntity struct {
	Mechanism     V5MechanismRuntime
	StaticLibrary V4Library
	Experience    *V7CognitiveAgent
	Memory        V5AdaptiveMemory
	Inquiry       InquiryManager
	Evidence      []V8CapabilityEvidence
	Version       uint64
	Failures      []string
}

func NewV8CognitiveEntity() *V8CognitiveEntity {
	return &V8CognitiveEntity{
		Mechanism:     *NewV5MechanismRuntime(),
		StaticLibrary: NewV4Library(),
		Experience:    NewV7CognitiveAgent(),
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
		e.attest("causal-inquiry", "v8-inquiry-"+plan.Action, "selected discriminating intervention", true)
		return nil
	}
	post, err := V5ReviseHypotheses(h, observedAction, observedOutcome)
	if err != nil {
		e.Failures = append(e.Failures, "causal-model-refuted:"+observedAction)
		return err
	}
	e.attest("causal-inquiry", "v8-belief-"+observedAction, fmt.Sprintf("surviving-hypotheses=%d", len(post)), true)
	return nil
}

func (e *V8CognitiveEntity) ObserveAndAct(state RelationalState, actions []string) (string, error) {
	if e == nil || e.Experience == nil {
		return "", errors.New("V8 experience core unavailable")
	}
	action, err := e.Experience.NextAction(state, actions)
	if err != nil {
		e.Failures = append(e.Failures, "action-selection:"+err.Error())
		return "", err
	}
	e.attest("interactive-action-selection", "v8-act-"+V7StateKey(state), "selected action from verified experience/novelty", true)
	return action, nil
}

func (e *V8CognitiveEntity) ObserveOutcome(before RelationalState, action string, after RelationalState, reward float64, terminal bool) V7Step {
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
	e.attest("mechanism-selection", "v8-semantic-strategy",
		fmt.Sprintf("hidden-ratios=%v", ratios), true)
	e.Version++
	return nil
}

func (e *V8CognitiveEntity) RollbackMechanism() error {
	if e == nil {
		return errors.New("nil V8 entity")
	}
	if err := e.Mechanism.Rollback(); err != nil {
		return err
	}
	e.Version = e.Mechanism.Version
	e.attest("rollback", fmt.Sprintf("v8-version-%d", e.Version),
		"previous executable mechanism restored", true)
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
