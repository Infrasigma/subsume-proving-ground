package acex

import "errors"

type Predictor interface {
	Predict(state NumericState, action string) PredictedTransition
}

type RepresentationGenerator interface {
	Discover(train, holdout Dataset) (Concept, Resource, error)
}

type SearchLanguageLearner interface {
	Search(train, holdout []Dataset, current RankerProgram) (RankerSearchResult, error)
}

type BeliefEngine interface {
	ChooseIntervention(beliefs []Belief, outcomes map[string]map[string]float64) (string, float64, error)
	Revise(beliefs []Belief, action, outcome string) ([]Belief, error)
}

type VerifierEngine interface {
	VerifyPrediction(before NumericState, action string, predicted, observed NumericState) bool
}

type EnvironmentEngine interface {
	Observe() NumericState
	Act(action string) (NumericState, error)
}

type PurePredictor struct {
	Model *PredictiveModel
}

func (p PurePredictor) Predict(state NumericState, action string) PredictedTransition {
	if p.Model == nil {
		return PredictedTransition{State:copyState(state),Known:false}
	}
	return p.Model.Predict(state,action)
}

type PureRepresentationGenerator struct {
	Lab RepresentationLab
}

func (p PureRepresentationGenerator) Discover(train, holdout Dataset) (Concept, Resource, error) {
	return p.Lab.Discover(train,holdout)
}

type PureSearchLanguageLearner struct{}

func (PureSearchLanguageLearner) Search(train, holdout []Dataset, current RankerProgram) (RankerSearchResult,error) {
	return SearchRankerProgram(train,holdout,current)
}

type PureBeliefEngine struct{}

func (PureBeliefEngine) ChooseIntervention(b []Belief, outcomes map[string]map[string]float64) (string,float64,error) {
	return (BeliefRevision{}).ChooseIntervention(b,outcomes)
}

func (PureBeliefEngine) Revise(b []Belief, action,outcome string) ([]Belief,error) {
	return (BeliefRevision{}).Revise(b,action,outcome)
}

type PureVerifier struct{}

func (PureVerifier) VerifyPrediction(before NumericState, action string, predicted, observed NumericState) bool {
	_ = before
	_ = action
	return stateKey(predicted)==stateKey(observed)
}

type ModelIndependentCore struct {
	Predictor     Predictor
	Representation RepresentationGenerator
	SearchLanguage SearchLanguageLearner
	Beliefs       BeliefEngine
	Verifier      VerifierEngine
}

func NewPureCore() *ModelIndependentCore {
	model:=NewPredictiveModel()
	return &ModelIndependentCore{
		Predictor:PurePredictor{Model:model},
		Representation:PureRepresentationGenerator{Lab:RepresentationLab{MaxAtoms:4,Policy:PolicyBroad}},
		SearchLanguage:PureSearchLanguageLearner{},
		Beliefs:PureBeliefEngine{},
		Verifier:PureVerifier{},
	}
}

func (c *ModelIndependentCore) Validate() error {
	if c==nil || c.Predictor==nil || c.Representation==nil || c.SearchLanguage==nil ||
		c.Beliefs==nil || c.Verifier==nil {
		return errors.New("core missing cognitive interface")
	}
	return nil
}
