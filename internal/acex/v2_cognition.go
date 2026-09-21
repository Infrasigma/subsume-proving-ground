package acex

import (
	"errors"
	"math"
	"sort"
)

type Belief struct {
	ID        string
	Prior     float64
	Posterior float64
	Predicted map[string]string
}

type BeliefRevision struct{}

func entropy(ps []float64) float64 {
	h := 0.0
	for _, p := range ps {
		if p > 0 {
			h -= p * math.Log2(p)
		}
	}
	return h
}

func NormalizeBeliefs(b []Belief) []Belief {
	out := append([]Belief(nil), b...)
	sum := 0.0
	for i := range out {
		sum += out[i].Prior
	}
	if sum <= 0 {
		sum = float64(len(out))
	}
	for i := range out {
		out[i].Posterior = out[i].Prior / sum
	}
	return out
}

// ChooseIntervention maximizes expected entropy reduction over the current
// belief distribution. The action/outcome table is supplied by the environment.
func (BeliefRevision) ChooseIntervention(b []Belief, outcomes map[string]map[string]float64) (string, float64, error) {
	if len(b) < 2 {
		return "", 0, errors.New("need competing beliefs")
	}
	b = NormalizeBeliefs(b)
	base := make([]float64, len(b))
	for i := range b {
		base[i] = b[i].Posterior
	}
	baseH := entropy(base)
	bestAction := ""
	bestGain := -1.0
	for action, byOutcome := range outcomes {
		expectedH := 0.0
		mass := 0.0
		for outcome, likelihood := range byOutcome {
			if likelihood <= 0 {
				continue
			}
			mass += likelihood
			post := make([]float64, len(b))
			z := 0.0
			for i, x := range b {
				p, ok := x.Predicted[action]
				if !ok {
					continue
				}
				if p == outcome {
					post[i] = x.Posterior
				}
				z += post[i]
			}
			if z > 0 {
				for i := range post {
					post[i] /= z
				}
				expectedH += likelihood * entropy(post)
			}
		}
		if mass > 0 {
			expectedH /= mass
		}
		gain := baseH - expectedH
		if gain > bestGain {
			bestGain = gain
			bestAction = action
		}
	}
	if bestAction == "" || bestGain <= 0 {
		return "", 0, errors.New("no informative intervention")
	}
	return bestAction, bestGain, nil
}

func (BeliefRevision) Revise(b []Belief, action, outcome string) ([]Belief, error) {
	if len(b) < 2 {
		return nil, errors.New("need competing beliefs")
	}
	out := append([]Belief(nil), b...)
	z := 0.0
	for i := range out {
		pred, ok := out[i].Predicted[action]
		if !ok {
			out[i].Posterior = 0
			continue
		}
		if pred == outcome {
			out[i].Posterior = out[i].Posterior
		} else {
			out[i].Posterior = 0
		}
		z += out[i].Posterior
	}
	if z <= 0 {
		return nil, errors.New("all beliefs falsified")
	}
	for i := range out {
		out[i].Posterior /= z
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Posterior > out[j].Posterior })
	return out, nil
}

type TraceStep struct {
	Op string
	Arg int
}

type Trace struct {
	Steps []TraceStep
}

type Macro struct {
	ID    string
	Steps []TraceStep
	Uses  int
}

func sameStep(a, b TraceStep) bool {
	return a.Op == b.Op && a.Arg == b.Arg
}

func commonSubtraces(traces []Trace, minLen int) []Macro {
	if len(traces) < 2 {
		return nil
	}
	seed := traces[0].Steps
	out := make([]Macro, 0)
	for start := 0; start < len(seed); start++ {
		for length := minLen; start+length <= len(seed); length++ {
			candidate := seed[start : start+length]
			uses := 1
			for ti := 1; ti < len(traces); ti++ {
				found := false
				for j := 0; j+length <= len(traces[ti].Steps); j++ {
					ok := true
					for k := 0; k < length; k++ {
						if !sameStep(candidate[k], traces[ti].Steps[j+k]) {
							ok = false
							break
						}
					}
					if ok {
						found = true
						break
					}
				}
				if found {
					uses++
				}
			}
			if uses >= 2 {
				x := append([]TraceStep(nil), candidate...)
				out = append(out, Macro{ID: "macro-" + string(rune('A'+len(out))), Steps: x, Uses: uses})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].Steps) != len(out[j].Steps) {
			return len(out[i].Steps) > len(out[j].Steps)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func ApplyMacro(trace Trace, macro Macro) Trace {
	out := Trace{Steps: append([]TraceStep(nil), trace.Steps...)}
	if len(macro.Steps) == 0 {
		return out
	}
	for i := 0; i+len(macro.Steps) <= len(out.Steps); {
		ok := true
		for j := range macro.Steps {
			if !sameStep(out.Steps[i+j], macro.Steps[j]) {
				ok = false
				break
			}
		}
		if !ok {
			i++
			continue
		}
		repl := []TraceStep{{Op: macro.ID, Arg: len(macro.Steps)}}
		tmp := make([]TraceStep, 0, len(out.Steps))
		tmp = append(tmp, out.Steps[:i]...)
		tmp = append(tmp, repl...)
		tmp = append(tmp, out.Steps[i+len(macro.Steps):]...)
		out.Steps = tmp
		i++
	}
	return out
}

func MacroDiscoveryCost(trace Trace, library []Macro) int {
	cost := len(trace.Steps)
	for _, m := range library {
		if len(m.Steps) > 1 {
			for i := 0; i+len(m.Steps) <= len(trace.Steps); i++ {
				ok := true
				for j := range m.Steps {
					if !sameStep(trace.Steps[i+j], m.Steps[j]) {
						ok = false
						break
					}
				}
				if ok {
					cost -= len(m.Steps) - 1
				}
			}
		}
	}
	if cost < 1 {
		cost = 1
	}
	return cost
}

type Gap struct {
	Name       string
	SearchFail bool
	ModelFail  bool
	TransferFail bool
}

type CurriculumDecision struct {
	Challenge string
	Reason    string
}

type CurriculumEngine struct{}

func (CurriculumEngine) SelectGap(gaps []Gap) (CurriculumDecision, error) {
	if len(gaps) == 0 {
		return CurriculumDecision{}, errors.New("no observed gaps")
	}
	for _, g := range gaps {
		if g.TransferFail {
			return CurriculumDecision{
				Challenge: "surface-shift-and-topology-change",
				Reason:    "transfer failure is the highest verified unresolved capability gap",
			}, nil
		}
	}
	for _, g := range gaps {
		if g.ModelFail {
			return CurriculumDecision{
				Challenge: "intervention-counterexample",
				Reason:    "predictive model failure requires discriminating intervention",
			}, nil
		}
	}
	for _, g := range gaps {
		if g.SearchFail {
			return CurriculumDecision{
				Challenge: "mechanism-invention",
				Reason:    "search failure requires new reusable mechanism",
			}, nil
		}
	}
	return CurriculumDecision{Challenge: "novel-composition", Reason: "all tracked gaps currently resolved"}, nil
}
