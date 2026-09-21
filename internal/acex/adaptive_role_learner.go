package acex

import (
	"fmt"
	"sort"
	"strings"
)

// AdaptiveRoleExample is a labeled action-centered observation. Identifiers,
// node kinds, attributes, and edge labels are deliberately excluded from the
// learned representation so the retained rule can survive surface changes.
type AdaptiveRoleExample struct {
	State    RelationalState
	Action   string
	Positive bool
}

// AdaptiveRoleLearner searches a progressively deeper rooted structural
// representation. It starts with radius-1 neighborhoods and increases depth
// only after the current representation cannot separate observed successes
// from observed failures.
//
// This is intentionally bounded: it is a live experiment for adaptive
// representation expansion, not a claim of open-ended representation
// invention.
type AdaptiveRoleLearner struct {
	MaxRadius       int
	Radius          int
	Ready           bool
	PositiveSig     string
	NegativeSigs    map[string]bool
	Examples        []AdaptiveRoleExample
	SearchExpansions int
}

func NewAdaptiveRoleLearner() AdaptiveRoleLearner {
	return AdaptiveRoleLearner{
		MaxRadius:    4,
		NegativeSigs: map[string]bool{},
	}
}

func (l *AdaptiveRoleLearner) Observe(state RelationalState, action string, positive bool) {
	if l.MaxRadius <= 0 {
		l.MaxRadius = 4
	}
	if l.NegativeSigs == nil {
		l.NegativeSigs = map[string]bool{}
	}
	l.Examples = append(l.Examples, AdaptiveRoleExample{
		State: state,
		Action: action,
		Positive: positive,
	})
	l.learn()
}

func (l *AdaptiveRoleLearner) learn() {
	if len(l.Examples) < 2 {
		return
	}

	for radius := 1; radius <= l.MaxRadius; radius++ {
		l.SearchExpansions++
		posCount := map[string]int{}
		negCount := map[string]int{}
		for _, ex := range l.Examples {
			sig, ok := rootedStructuralSignature(ex.State, ex.Action, radius)
			if !ok {
				continue
			}
			if ex.Positive {
				posCount[sig]++
			} else {
				negCount[sig]++
			}
		}

		candidates := make([]string, 0, len(posCount))
		for sig := range posCount {
			if negCount[sig] == 0 {
				candidates = append(candidates, sig)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		sort.Strings(candidates)
		chosen := candidates[0]
		l.Radius = radius
		l.PositiveSig = chosen
		l.NegativeSigs = map[string]bool{}
		for sig := range negCount {
			if sig != chosen {
				l.NegativeSigs[sig] = true
			}
		}
		l.Ready = true
		return
	}
}

func (l AdaptiveRoleLearner) Predict(state RelationalState, actions []string) (string, bool) {
	if !l.Ready || l.PositiveSig == "" {
		return "", false
	}
	matches := make([]string, 0, len(actions))
	for _, action := range actions {
		sig, ok := rootedStructuralSignature(state, action, l.Radius)
		if !ok || sig != l.PositiveSig || l.NegativeSigs[sig] {
			continue
		}
		matches = append(matches, action)
	}
	if len(matches) == 0 {
		return "", false
	}
	sort.Strings(matches)
	return matches[0], true
}

func rootedStructuralSignature(state RelationalState, root string, radius int) (string, bool) {
	if radius < 0 {
		return "", false
	}
	present := map[string]bool{}
	for _, n := range state.Nodes {
		present[n.ID] = true
	}
	if !present[root] {
		return "", false
	}

	adj := map[string][]string{}
	for _, e := range state.Edges {
		if !present[e.From] || !present[e.To] {
			continue
		}
		adj[e.From] = append(adj[e.From], e.To)
		adj[e.To] = append(adj[e.To], e.From)
	}

	dist := map[string]int{root: 0}
	queue := []string{root}
	for head := 0; head < len(queue); head++ {
		cur := queue[head]
		if dist[cur] >= radius {
			continue
		}
		for _, next := range adj[cur] {
			if _, seen := dist[next]; seen {
				continue
			}
			dist[next] = dist[cur] + 1
			queue = append(queue, next)
		}
	}

	local := map[string]bool{}
	for id := range dist {
		local[id] = true
	}

	degree := map[string]int{}
	edgeCount := 0
	seenEdges := map[string]bool{}
	for _, e := range state.Edges {
		if !local[e.From] || !local[e.To] {
			continue
		}
		degree[e.From]++
		degree[e.To]++
		key := e.From + "\x00" + e.To + "\x00" + e.Kind
		if !seenEdges[key] {
			seenEdges[key] = true
			edgeCount++
		}
	}

	parts := make([]string, 0, len(local))
	for id := range local {
		parts = append(parts, fmt.Sprintf("%d:%d", dist[id], degree[id]))
	}
	sort.Strings(parts)
	return fmt.Sprintf("r=%d|n=%d|e=%d|%s", radius, len(local), edgeCount, strings.Join(parts, ",")), true
}
