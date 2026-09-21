package acex

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
)

type NumericState map[string]int

func copyState(s NumericState) NumericState {
	out := NumericState{}
	for k, v := range s {
		out[k] = v
	}
	return out
}

type Transition struct {
	Before NumericState
	Action string
	After  NumericState
}

type PredictedTransition struct {
	State      NumericState
	Confidence float64
	Known      bool
}

type ActionModel struct {
	Count int
	Delta map[string]int
	Mismatch int
}

type PredictiveModel struct {
	Actions map[string]*ActionModel
}

func NewPredictiveModel() *PredictiveModel {
	return &PredictiveModel{Actions: map[string]*ActionModel{}}
}

func stateDelta(before, after NumericState) map[string]int {
	d := map[string]int{}
	keys := map[string]bool{}
	for k := range before { keys[k] = true }
	for k := range after { keys[k] = true }
	for k := range keys {
		d[k] = after[k] - before[k]
	}
	return d
}

func (m *PredictiveModel) Observe(t Transition) error {
	if m == nil {
		return errors.New("nil predictive model")
	}
	if m.Actions[t.Action] == nil {
		m.Actions[t.Action] = &ActionModel{Delta: map[string]int{}}
	}
	a := m.Actions[t.Action]
	d := stateDelta(t.Before, t.After)
	a.Count++
	for k, v := range d {
		old := a.Delta[k]
		a.Delta[k] = int(math.Round(float64(old*(a.Count-1)+v) / float64(a.Count)))
	}
	return nil
}

func (m *PredictiveModel) Predict(s NumericState, action string) PredictedTransition {
	a, ok := m.Actions[action]
	if !ok || a.Count == 0 {
		return PredictedTransition{State: copyState(s), Confidence: 0, Known: false}
	}
	out := copyState(s)
	for k, d := range a.Delta {
		out[k] += d
	}
	conf := float64(a.Count) / float64(a.Count+1)
	if a.Mismatch > 0 {
		conf *= 1 / float64(a.Mismatch+1)
	}
	return PredictedTransition{State: out, Confidence: conf, Known: true}
}

func (m *PredictiveModel) VerifyAndRevise(t Transition) bool {
	p := m.Predict(t.Before, t.Action)
	if !p.Known {
		_ = m.Observe(t)
		return true
	}
	ok := true
	for k, v := range t.After {
		if p.State[k] != v {
			ok = false
			break
		}
	}
	if !ok {
		m.Actions[t.Action].Mismatch++
		// Keep the surprising observation as evidence for a future
		// representation split rather than silently trusting the stale rule.
	}
	_ = m.Observe(t)
	return ok
}

type Goal func(NumericState) bool

type PlanResult struct {
	Actions []string
	State   NumericState
	Cost    int
	Confidence float64
}

func PlanWithForesight(m *PredictiveModel, start NumericState, actions []string, goal Goal, horizon int, minConfidence float64) (PlanResult, error) {
	if m == nil || goal == nil {
		return PlanResult{}, errors.New("missing model or goal")
	}
	type node struct {
		state NumericState
		path []string
		conf float64
	}
	q := []node{{state:copyState(start),conf:1}}
	seen := map[string]bool{}
	for depth := 0; depth <= horizon; depth++ {
		next := make([]node,0,len(q)*len(actions))
		for _, n := range q {
			if goal(n.state) {
				return PlanResult{Actions:n.path,State:n.state,Cost:len(n.path),Confidence:n.conf},nil
			}
			for _, action := range actions {
				p := m.Predict(n.state, action)
				if !p.Known || p.Confidence < minConfidence {
					continue
				}
				key := stateKey(p.State)+"|"+strings.Join(append(n.path,action),",")
				if seen[key] {
					continue
				}
				seen[key] = true
				path := append(append([]string(nil), n.path...), action)
				next = append(next,node{state:p.State,path:path,conf:math.Min(n.conf,p.Confidence)})
			}
		}
		q = next
	}
	return PlanResult{}, errors.New("no high-confidence plan")
}

func stateKey(s NumericState) string {
	keys := make([]string,0,len(s))
	for k := range s { keys=append(keys,k) }
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(strconv.Itoa(s[k]))
		b.WriteByte(';')
	}
	return b.String()
}
