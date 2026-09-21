package acex

import (
	"fmt"
	"sort"
)

// V8StructuralRoleLearner learns action roles from label-invariant graph structure.
// It deliberately ignores node kinds, attribute names/values, edge kinds, and
// concrete identifiers. This is a bounded transfer mechanism, not open-ended
// representation invention.
type V8StructuralRoleLearner struct {
	Success map[string]int
	Failure map[string]int
}

func NewV8StructuralRoleLearner() V8StructuralRoleLearner {
	return V8StructuralRoleLearner{
		Success: map[string]int{},
		Failure: map[string]int{},
	}
}

func structuralDegrees(g RelationalState) map[string]int {
	deg := make(map[string]int, len(g.Nodes))
	for _, n := range g.Nodes {
		deg[n.ID] = 0
	}
	for _, e := range g.Edges {
		if _, ok := deg[e.From]; ok {
			deg[e.From]++
		}
		if _, ok := deg[e.To]; ok {
			deg[e.To]++
		}
	}
	return deg
}

func StructuralActionRoleKey(g RelationalState, action string) (string, bool) {
	found := false
	for _, n := range g.Nodes {
		if n.ID == action {
			found = true
			break
		}
	}
	if !found {
		return "", false
	}

	deg := structuralDegrees(g)
	actionDegree := deg[action]

	neighborIDs := map[string]bool{}
	for _, e := range g.Edges {
		switch {
		case e.From == action && e.To != action:
			neighborIDs[e.To] = true
		case e.To == action && e.From != action:
			neighborIDs[e.From] = true
		}
	}

	maxNeighborDegree := 0
	neighborDegreeSum := 0
	for id := range neighborIDs {
		d := deg[id]
		if d > maxNeighborDegree {
			maxNeighborDegree = d
		}
		neighborDegreeSum += d
	}

	nonActionNeighborCount := len(neighborIDs)

	// Keep the representation small and entirely structural.
	return fmt.Sprintf("d=%d|nd=%d|ns=%d", actionDegree, maxNeighborDegree, nonActionNeighborCount), true
}

func (l *V8StructuralRoleLearner) Observe(g RelationalState, action string, reward float64, terminal bool) {
	if l == nil {
		return
	}
	key, ok := StructuralActionRoleKey(g, action)
	if !ok {
		return
	}
	if reward > 0 || terminal {
		l.Success[key]++
	} else if reward < 0 {
		l.Failure[key]++
	}
}

func (l *V8StructuralRoleLearner) Select(g RelationalState, actions []string) (string, bool) {
	if l == nil || len(actions) == 0 {
		return "", false
	}
	type candidate struct {
		action string
		score  float64
		succ   int
		fail   int
	}
	cs := make([]candidate, 0, len(actions))
	for _, action := range actions {
		key, ok := StructuralActionRoleKey(g, action)
		if !ok {
			continue
		}
		succ := l.Success[key]
		fail := l.Failure[key]
		// A representation that has ever mapped both positive and
		// negative outcomes is not safe for transfer. Fail closed rather
		// than majority-voting over a conflicted abstraction.
		if succ == 0 || fail > 0 {
			continue
		}
		score := float64(succ) / float64(succ+fail)
		cs = append(cs, candidate{action: action, score: score, succ: succ, fail: fail})
	}
	sort.SliceStable(cs, func(i, j int) bool {
		if cs[i].score != cs[j].score {
			return cs[i].score > cs[j].score
		}
		if cs[i].succ != cs[j].succ {
			return cs[i].succ > cs[j].succ
		}
		return cs[i].action < cs[j].action
	})
	if len(cs) == 0 {
		return "", false
	}
	return cs[0].action, true
}
