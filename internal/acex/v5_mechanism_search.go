package acex

import (
	"errors"
	"sort"
)

type V5MechanismSearchResult struct {
	Candidate V5Mechanism
	Visible   V5MechanismResult
	Ranked    []V5MechanismResult
}

// V5SelectMechanism is a meta-search over cognitive mechanisms. The selector
// sees visible outcomes only; hidden tasks remain evaluator-owned.
func V5SelectMechanism(base V5Mechanism, visible []V5Task, gap string) (V5MechanismSearchResult, error) {
	candidates := V5MechanismCandidates(base, gap)
	ranked := make([]V5MechanismResult, 0, len(candidates))
	for _, c := range candidates {
		res := V5EvaluateMechanism(c, visible)
		if res.Verified {
			ranked = append(ranked, res)
		}
	}
	if len(ranked) == 0 {
		return V5MechanismSearchResult{}, errors.New("mechanism search found no verified candidate")
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].TotalCost != ranked[j].TotalCost {
			return ranked[i].TotalCost < ranked[j].TotalCost
		}
		return ranked[i].Mechanism.Key() < ranked[j].Mechanism.Key()
	})
	baseRes := V5EvaluateMechanism(base, visible)
	if !baseRes.Verified || ranked[0].TotalCost >= baseRes.TotalCost {
		return V5MechanismSearchResult{}, errors.New("mechanism search found no verified improvement")
	}
	return V5MechanismSearchResult{
		Candidate: ranked[0].Mechanism,
		Visible:   ranked[0],
		Ranked:    ranked,
	}, nil
}
