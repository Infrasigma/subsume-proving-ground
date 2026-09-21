package acex

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type V8AdaptiveRoleExample struct {
	State   RelationalState
	Action  string
	Success bool
}

// V8AdaptiveStructuralRoleLearner searches a growing family of generic,
// label-invariant rooted graph representations. It starts with radius 1 and
// increases radius only when the current representation collides positive and
// negative experience. This is bounded representation invention, not yet
// open-ended representation invention.
type V8AdaptiveStructuralRoleLearner struct {
	ActiveRadius int
	MaxRadius    int
	Examples     []V8AdaptiveRoleExample
	Success      map[string]int
	Failure      map[string]int
}

func NewV8AdaptiveStructuralRoleLearner() V8AdaptiveStructuralRoleLearner {
	return V8AdaptiveStructuralRoleLearner{
		ActiveRadius: 1,
		MaxRadius:    3,
		Success:      map[string]int{},
		Failure:      map[string]int{},
	}
}

func rootedNeighborhoodKey(g RelationalState, root string, radius int) (string, bool) {
	if radius < 0 {
		return "", false
	}
	adj := map[string][]string{}
	present := map[string]bool{}
	for _, n := range g.Nodes {
		present[n.ID] = true
	}
	for _, e := range g.Edges {
		if present[e.From] && present[e.To] {
			adj[e.From] = append(adj[e.From], e.To)
			adj[e.To] = append(adj[e.To], e.From)
		}
	}
	if !present[root] {
		return "", false
	}
	dist := map[string]int{root: 0}
	q := []string{root}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		if dist[cur] >= radius {
			continue
		}
		for _, nxt := range adj[cur] {
			if _, ok := dist[nxt]; ok {
				continue
			}
			dist[nxt] = dist[cur] + 1
			q = append(q, nxt)
		}
	}

	layerCounts := make([]int, radius+1)
	for _, d := range dist {
		if d <= radius {
			layerCounts[d]++
		}
	}
	edgeLayers := make([]int, 0, (radius+1)*(radius+2)/2)
	for lo := 0; lo <= radius; lo++ {
		for hi := lo; hi <= radius; hi++ {
			count := 0
			seen := map[[2]string]bool{}
			for _, e := range g.Edges {
				a, oka := dist[e.From]
				b, okb := dist[e.To]
				if !oka || !okb || a > radius || b > radius {
					continue
				}
				x, y := e.From, e.To
				if x > y {
					x, y = y, x
				}
				pair := [2]string{x, y}
				if seen[pair] {
					continue
				}
				seen[pair] = true
				minD, maxD := a, b
				if minD > maxD {
					minD, maxD = maxD, minD
				}
				if minD == lo && maxD == hi {
					count++
				}
			}
			edgeLayers = append(edgeLayers, count)
		}
	}
	parts := make([]string, 0, 2+len(layerCounts)+len(edgeLayers))
	parts = append(parts, "r="+strconv.Itoa(radius))
	parts = append(parts, "n="+joinInts(layerCounts))
	parts = append(parts, "e="+joinInts(edgeLayers))
	return strings.Join(parts, "|"), true
}

func joinInts(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return strings.Join(parts, ",")
}

func (l *V8AdaptiveStructuralRoleLearner) rebuild() {
	if l == nil {
		return
	}
	l.Success = map[string]int{}
	l.Failure = map[string]int{}
	for _, ex := range l.Examples {
		key, ok := rootedNeighborhoodKey(ex.State, ex.Action, l.ActiveRadius)
		if !ok {
			continue
		}
		if ex.Success {
			l.Success[key]++
		} else {
			l.Failure[key]++
		}
	}
}

func (l *V8AdaptiveStructuralRoleLearner) representationSeparates() bool {
	if l == nil || len(l.Success) == 0 || len(l.Failure) == 0 {
		return false
	}
	for key := range l.Success {
		if l.Failure[key] > 0 {
			return false
		}
	}
	return true
}

func (l *V8AdaptiveStructuralRoleLearner) inventRepresentation() {
	if l == nil {
		return
	}
	if l.MaxRadius <= 0 {
		l.MaxRadius = 3
	}
	old := l.ActiveRadius
	for radius := maxIntV8(1, old+1); radius <= l.MaxRadius; radius++ {
		l.ActiveRadius = radius
		l.rebuild()
		if l.representationSeparates() {
			return
		}
	}
	l.ActiveRadius = old
	l.rebuild()
}

func (l *V8AdaptiveStructuralRoleLearner) Observe(state RelationalState, action string, reward float64, terminal bool) {
	if l == nil {
		return
	}
	key, ok := rootedNeighborhoodKey(state, action, l.ActiveRadius)
	_ = key
	if !ok {
		return
	}
	l.Examples = append(l.Examples, V8AdaptiveRoleExample{
		State:   cloneRelationalState(state),
		Action:  action,
		Success: reward > 0 || terminal,
	})
	l.rebuild()
	if !l.representationSeparates() {
		l.inventRepresentation()
	}
}

func (l *V8AdaptiveStructuralRoleLearner) Select(state RelationalState, actions []string) (string, bool) {
	if l == nil || len(actions) == 0 || len(l.Success) == 0 {
		return "", false
	}
	if !l.representationSeparates() {
		l.inventRepresentation()
	}
	type cand struct {
		action string
		score  float64
		succ   int
	}
	cs := make([]cand, 0, len(actions))
	for _, action := range actions {
		key, ok := rootedNeighborhoodKey(state, action, l.ActiveRadius)
		if !ok {
			continue
		}
		succ := l.Success[key]
		fail := l.Failure[key]
		if succ == 0 || fail > 0 {
			continue
		}
		cs = append(cs, cand{
			action: action,
			score: float64(succ) / float64(succ+fail),
			succ:   succ,
		})
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

func (l V8AdaptiveStructuralRoleLearner) InventedRepresentation() string {
	return fmt.Sprintf("rooted-neighborhood-radius-%d", l.ActiveRadius)
}

func maxIntV8(a, b int) int {
	if a > b {
		return a
	}
	return b
}
