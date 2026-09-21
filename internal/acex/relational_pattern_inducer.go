package acex

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type V8RelationalPatternEdge struct {
	A int
	B int
}

type V8RelationalPattern struct {
	Radius int
	Nodes  int
	Edges  []V8RelationalPatternEdge
}

type V8RelationalPatternExample struct {
	State   RelationalState
	Action  string
	Success bool
}

// V8RelationalPatternInducer is a generic bounded search over rooted
// relational structures. It is deliberately not a menu of named target
// features: the executable representation is an induced graph pattern that
// is synthesized from positive/negative experience and retained independently
// of the raw episodes.
//
// This is a representation-synthesis mechanism, not yet evidence of
// open-ended mechanism invention.
type V8RelationalPatternInducer struct {
	MaxRadius       int
	MaxNodes        int
	Budget          int
	SearchExpansions int
	Examples        []V8RelationalPatternExample
	Pattern         *V8RelationalPattern
}

func NewV8RelationalPatternInducer() V8RelationalPatternInducer {
	return V8RelationalPatternInducer{
		MaxRadius: 3,
		MaxNodes:  5,
		Budget:    6000,
	}
}

func (p V8RelationalPattern) Complexity() int {
	return p.Radius + p.Nodes + len(p.Edges)
}

func (p V8RelationalPattern) Key() string {
	pairs := make([]string, 0, len(p.Edges))
	for _, e := range p.Edges {
		a, b := e.A, e.B
		if a > b {
			a, b = b, a
		}
		pairs = append(pairs, fmt.Sprintf("%d-%d", a, b))
	}
	sort.Strings(pairs)
	return fmt.Sprintf("r=%d|n=%d|e=%s", p.Radius, p.Nodes, strings.Join(pairs, ","))
}

func (p V8RelationalPattern) Separates(examples []V8RelationalPatternExample) bool {
	if len(examples) == 0 {
		return false
	}
	seenPositive := false
	seenNegative := false
	for _, ex := range examples {
		matched := p.Match(ex.State, ex.Action)
		if ex.Success {
			seenPositive = true
			if !matched {
				return false
			}
		} else {
			seenNegative = true
			if matched {
				return false
			}
		}
	}
	return seenPositive && seenNegative
}

func (p V8RelationalPattern) Match(state RelationalState, root string) bool {
	if p.Nodes <= 0 || p.Radius < 0 {
		return false
	}
	if p.Nodes == 1 {
		for _, n := range state.Nodes {
			if n.ID == root {
				return true
			}
		}
		return false
	}

	local, adj, ok := rootedLocalGraph(state, root, p.Radius)
	if !ok || len(local) < p.Nodes {
		return false
	}

	pdeg := make([]int, p.Nodes)
	patternAdj := make([]map[int]bool, p.Nodes)
	for i := range patternAdj {
		patternAdj[i] = map[int]bool{}
	}
	for _, e := range p.Edges {
		if e.A < 0 || e.B < 0 || e.A >= p.Nodes || e.B >= p.Nodes || e.A == e.B {
			return false
		}
		if patternAdj[e.A][e.B] {
			continue
		}
		patternAdj[e.A][e.B] = true
		patternAdj[e.B][e.A] = true
		pdeg[e.A]++
		pdeg[e.B]++
	}

	order := make([]int, 0, p.Nodes-1)
	for i := 1; i < p.Nodes; i++ {
		order = append(order, i)
	}
	sort.SliceStable(order, func(i, j int) bool {
		if pdeg[order[i]] != pdeg[order[j]] {
			return pdeg[order[i]] > pdeg[order[j]]
		}
		return order[i] < order[j]
	})

	mapping := make([]string, p.Nodes)
	used := map[string]bool{root:true}
	mapping[0] = root

	var search func(int) bool
	search = func(pos int) bool {
		if pos == len(order) {
			return true
		}
		pv := order[pos]
		for _, gv := range local {
			if used[gv] || len(adj[gv]) < pdeg[pv] {
				continue
			}
			valid := true
			for q := 0; q < p.Nodes; q++ {
				if mapping[q] == "" || !patternAdj[pv][q] {
					continue
				}
				if !adj[gv][mapping[q]] {
					valid = false
					break
				}
			}
			if !valid {
				continue
			}
			mapping[pv] = gv
			used[gv] = true
			if search(pos + 1) {
				return true
			}
			delete(used, gv)
			mapping[pv] = ""
		}
		return false
	}

	return search(0)
}

func rootedLocalGraph(state RelationalState, root string, radius int) ([]string, map[string]map[string]bool, bool) {
	present := map[string]bool{}
	for _, n := range state.Nodes {
		present[n.ID] = true
	}
	if !present[root] {
		return nil, nil, false
	}
	adj := map[string]map[string]bool{}
	for _, n := range state.Nodes {
		adj[n.ID] = map[string]bool{}
	}
	for _, e := range state.Edges {
		if !present[e.From] || !present[e.To] {
			continue
		}
		adj[e.From][e.To] = true
		adj[e.To][e.From] = true
	}
	dist := map[string]int{root:0}
	queue := []string{root}
	for head := 0; head < len(queue); head++ {
		cur := queue[head]
		if dist[cur] >= radius {
			continue
		}
		for nxt := range adj[cur] {
			if _, ok := dist[nxt]; ok {
				continue
			}
			dist[nxt] = dist[cur] + 1
			queue = append(queue, nxt)
		}
	}
	local := make([]string, 0, len(dist))
	for id := range dist {
		local = append(local, id)
	}
	sort.Strings(local)
	localSet := map[string]bool{}
	for _, id := range local {
		localSet[id] = true
	}
	for _, id := range local {
		for neighbor := range adj[id] {
			if !localSet[neighbor] {
				delete(adj[id], neighbor)
			}
		}
	}
	return local, adj, true
}

func v8EnumerateRelationalPatterns(maxRadius, maxNodes int) []V8RelationalPattern {
	if maxRadius < 1 {
		maxRadius = 1
	}
	if maxNodes < 1 {
		maxNodes = 1
	}
	out := []V8RelationalPattern{}
	for radius := 1; radius <= maxRadius; radius++ {
		for nodes := 1; nodes <= maxNodes; nodes++ {
			if nodes == 1 {
				out = append(out, V8RelationalPattern{Radius:radius, Nodes:1})
				continue
			}
			pairs := make([]V8RelationalPatternEdge, 0, nodes*(nodes-1)/2)
			for a := 0; a < nodes; a++ {
				for b := a + 1; b < nodes; b++ {
					pairs = append(pairs, V8RelationalPatternEdge{A:a, B:b})
				}
			}
			masks := 1 << len(pairs)
			if len(pairs) >= 63 {
				continue
			}
			for mask := 0; mask < masks; mask++ {
				if bitsSet(mask) < nodes-1 {
					continue
				}
				edges := make([]V8RelationalPatternEdge, 0, bitsSet(mask))
				for i, e := range pairs {
					if mask&(1<<i) != 0 {
						edges = append(edges, e)
					}
				}
				if !patternConnected(nodes, edges) {
					continue
				}
				out = append(out, V8RelationalPattern{Radius:radius, Nodes:nodes, Edges:edges})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Complexity() != out[j].Complexity() {
			return out[i].Complexity() < out[j].Complexity()
		}
		return out[i].Key() < out[j].Key()
	})
	return out
}

func bitsSet(x int) int {
	count := 0
	for x != 0 {
		x &= x - 1
		count++
	}
	return count
}

func patternConnected(nodes int, edges []V8RelationalPatternEdge) bool {
	if nodes <= 1 {
		return true
	}
	adj := make([][]int, nodes)
	for _, e := range edges {
		adj[e.A] = append(adj[e.A], e.B)
		adj[e.B] = append(adj[e.B], e.A)
	}
	seen := map[int]bool{0:true}
	q := []int{0}
	for head := 0; head < len(q); head++ {
		for _, nxt := range adj[q[head]] {
			if seen[nxt] {
				continue
			}
			seen[nxt] = true
			q = append(q, nxt)
		}
	}
	return len(seen) == nodes
}

func (l *V8RelationalPatternInducer) Record(state RelationalState, action string, reward float64, terminal bool) {
	if l == nil {
		return
	}
	l.Examples = append(l.Examples, V8RelationalPatternExample{
		State: cloneRelationalState(state),
		Action: action,
		Success: reward > 0 || terminal,
	})
}

func (l *V8RelationalPatternInducer) TrySynthesize() bool {
	if l == nil || l.Pattern != nil || l.Budget <= 0 {
		return false
	}
	hasPositive, hasNegative := false, false
	for _, ex := range l.Examples {
		if ex.Success {
			hasPositive = true
		} else {
			hasNegative = true
		}
	}
	if !hasPositive || !hasNegative {
		return false
	}
	patterns := v8EnumerateRelationalPatterns(l.MaxRadius, l.MaxNodes)
	for _, p := range patterns {
		if l.SearchExpansions >= l.Budget {
			break
		}
		l.SearchExpansions++
		if p.Separates(l.Examples) {
			candidate := p
			l.Pattern = &candidate
			return true
		}
	}
	return false
}

func (l V8RelationalPatternInducer) Select(state RelationalState, actions []string) (string, bool) {
	if l.Pattern == nil {
		return "", false
	}
	matches := make([]string, 0, len(actions))
	for _, action := range actions {
		if l.Pattern.Match(state, action) {
			matches = append(matches, action)
		}
	}
	if len(matches) == 0 {
		return "", false
	}
	sort.Strings(matches)
	return matches[0], true
}

func (l *V8RelationalPatternInducer) ForgetExamples() {
	if l == nil {
		return
	}
	l.Examples = nil
}
