package acex

import (
	"errors"
	"fmt"
)

type World interface {
	Observe() NumericState
	Act(action string) (NumericState, error)
}

type RuntimeSelfModel struct {
	KnownActions map[string]float64
	Failures     []string
	Capabilities map[string]float64
	ResourceUsed Resource
}

type CognitiveRuntime struct {
	Memory      MemoryManager
	Model       *PredictiveModel
	Self        RuntimeSelfModel
	Executive   ExecutiveController
	Attention   AttentionController
	Relational  RelationalMemory
	Mechanisms  []Macro
	ActiveRanker RankerProgram
	RankerHistory []RankerProgram
	ImprovementLedger ImprovementLedger
	V3Library V3Library
	V3LibraryHistory []V3Library
	V4Library V4Library
	V4LibraryHistory []V4Library
	Version     uint64
}

func NewCognitiveRuntime() *CognitiveRuntime {
	return &CognitiveRuntime{
		Model: NewPredictiveModel(),
		ActiveRanker: RankerProgram{Expr:&RankExpr{Kind:"metric",Value:metricID("balance")}},
		Self: RuntimeSelfModel{
			KnownActions: map[string]float64{},
			Capabilities: map[string]float64{},
		},
		V3Library: NewV3Library(),
		V4Library: NewV4Library(),
	}
}

func (r *CognitiveRuntime) LearnTransition(before NumericState, action string, after NumericState) error {
	tr := Transition{Before: copyState(before), Action: action, After: copyState(after)}
	ok := r.Model.VerifyAndRevise(tr)
	if !ok {
		r.Self.Failures = append(r.Self.Failures, "prediction-mismatch:"+action)
	}
	p := r.Model.Predict(before, action)
	r.Self.KnownActions[action] = p.Confidence
	r.Self.Capabilities["predict:"+action] = p.Confidence
	return nil
}

func (r *CognitiveRuntime) Plan(start NumericState, actions []string, goal Goal) (PlanResult, error) {
	if r.Model == nil {
		return PlanResult{}, errors.New("runtime has no predictive model")
	}
	return PlanWithForesight(r.Model, start, actions, goal, 8, 0.50)
}

func (r *CognitiveRuntime) ExecutePlan(w World, start NumericState, plan PlanResult) (NumericState, error) {
	current := copyState(start)
	for _, action := range plan.Actions {
		next, err := w.Act(action)
		if err != nil {
			r.Self.Failures = append(r.Self.Failures, "execution:"+action)
			return current, err
		}
		before := copyState(current)
		pred := r.Model.Predict(before, action)
		if !pred.Known || pred.Confidence < 0.50 {
			r.Self.Failures = append(r.Self.Failures, "low-confidence:"+action)
		}
		if pred.Known && stateKey(pred.State) != stateKey(next) {
			r.Self.Failures = append(r.Self.Failures, "prediction-mismatch:"+action)
			r.Model.VerifyAndRevise(Transition{Before:before, Action:action, After:next})
		} else {
			r.Model.VerifyAndRevise(Transition{Before:before, Action:action, After:next})
		}
		current = copyState(next)
		r.Self.ResourceUsed.Search++
		r.Self.ResourceUsed.Verify++
	}
	return current, nil
}

func (r *CognitiveRuntime) ConsolidateTrace(traces []Trace, concept Concept) error {
	if len(traces) < 2 {
		return errors.New("need repeated traces")
	}
	item, cost, err := r.Memory.Consolidate(traces, concept, ProceduralMemory, "runtime")
	if err != nil {
		return err
	}
	if err := r.Memory.Add(item); err != nil {
		return err
	}
	r.Self.ResourceUsed.Search += cost.Search
	r.Self.ResourceUsed.Memory += cost.Memory
	r.Self.ResourceUsed.Storage += cost.Storage

	candidates := commonSubtraces(traces, 2)
	if len(candidates) > 0 {
		r.Mechanisms = append(r.Mechanisms, candidates[0])
	}
	return nil
}

func (r *CognitiveRuntime) Diagnose() string {
	if len(r.Self.Failures) == 0 {
		return "stable"
	}
	return fmt.Sprintf("failures=%d", len(r.Self.Failures))
}


func (r *CognitiveRuntime) ImproveSearchLanguage(train, holdout, hidden []Dataset) (RankerSearchResult, error) {
	if len(train) == 0 || len(train) != len(holdout) || len(hidden) == 0 {
		return RankerSearchResult{}, errors.New("invalid search-language improvement datasets")
	}
	candidate, err := SearchRankerProgram(train, holdout, r.ActiveRanker)
	if err != nil {
		return RankerSearchResult{}, err
	}
	for i := range hidden {
		baseOrdered := candidateFeatures(hidden[i])
		_, baseCost, baseErr := DiscoverWithOrder(hidden[i], hidden[i], baseOrdered, 4)
		_, newCost, newErr := DiscoverWithOrder(hidden[i], hidden[i], candidate.Program.OrderFixed(hidden[i]), 4)
		if baseErr != nil || newErr != nil || newCost.Total() >= baseCost.Total() {
			return RankerSearchResult{}, errors.New("candidate search language failed independent hidden verification")
		}
	}
	receipt:=MakeImprovementReceipt(r.Version,r.Version+1,candidate.Program.Signature(),candidate.Baseline,candidate.Improved,true)
	if err:=r.ImprovementLedger.Admit(receipt); err!=nil {
		return RankerSearchResult{},err
	}
	r.RankerHistory=append(r.RankerHistory,r.ActiveRanker)
	r.ActiveRanker = candidate.Program
	r.Version++
	return candidate, nil
}

func (r *CognitiveRuntime) RollbackSearchLanguage() error {
	if len(r.RankerHistory)==0 {
		return errors.New("no search-language rollback available")
	}
	r.ActiveRanker=r.RankerHistory[len(r.RankerHistory)-1]
	r.RankerHistory=r.RankerHistory[:len(r.RankerHistory)-1]
	if r.Version>0 { r.Version-- }
	return nil
}
