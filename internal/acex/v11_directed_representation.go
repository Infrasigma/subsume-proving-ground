package acex

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

type V11DirectedPatternEdge struct {
	From int `json:"from"`
	To   int `json:"to"`
}

type V11DirectedPatternArtifact struct {
	Version int                      `json:"version"`
	Root    int                      `json:"root"`
	Nodes   int                      `json:"nodes"`
	Edges   []V11DirectedPatternEdge  `json:"edges"`
}

type V11DirectedExecutableRepresentation struct {
	Root            int
	Nodes           int
	Edges           []V11DirectedPatternEdge
	Examples        []V8RelationalPatternExample
	SearchExpansions int
	Budget          int
	Valid           bool
	Retained        bool
}

func NewV11DirectedExecutableRepresentation() V11DirectedExecutableRepresentation {
	return V11DirectedExecutableRepresentation{
		Root:   0,
		Nodes:  0,
		Budget: 20000,
	}
}

func (p V11DirectedExecutableRepresentation) Complexity() int {
	return p.Nodes + len(p.Edges)
}

func (p V11DirectedExecutableRepresentation) Key() string {
	parts := make([]string, 0, len(p.Edges))
	for _, e := range p.Edges {
		parts = append(parts, fmt.Sprintf("%d>%d", e.From, e.To))
	}
	sort.Strings(parts)
	return fmt.Sprintf("root=%d|nodes=%d|edges=%s", p.Root, p.Nodes, fmt.Sprint(parts))
}

func v11UndirectedConnected(nodes int, edges []V11DirectedPatternEdge) bool {
	if nodes <= 1 {
		return true
	}
	adj := make([][]int, nodes)
	for _, e := range edges {
		if e.From < 0 || e.From >= nodes || e.To < 0 || e.To >= nodes || e.From == e.To {
			return false
		}
		adj[e.From] = append(adj[e.From], e.To)
		adj[e.To] = append(adj[e.To], e.From)
	}
	seen := map[int]bool{0:true}
	q := []int{0}
	for head := 0; head < len(q); head++ {
		for _, nxt := range adj[q[head]] {
			if !seen[nxt] {
				seen[nxt] = true
				q = append(q, nxt)
			}
		}
	}
	return len(seen) == nodes
}

func v11LocalNodes(state RelationalState, root string, radius int) map[string]bool {
	present := map[string]bool{}
	adj := map[string][]string{}
	for _, n := range state.Nodes {
		present[n.ID] = true
		adj[n.ID] = nil
	}
	if !present[root] {
		return nil
	}
	for _, e := range state.Edges {
		if !present[e.From] || !present[e.To] {
			continue
		}
		adj[e.From] = append(adj[e.From], e.To)
		adj[e.To] = append(adj[e.To], e.From)
	}
	dist := map[string]int{root:0}
	q := []string{root}
	for head := 0; head < len(q); head++ {
		cur := q[head]
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
	local := map[string]bool{}
	for id := range dist {
		local[id] = true
	}
	return local
}

func (p V11DirectedExecutableRepresentation) Match(state RelationalState, root string) bool {
	if !p.Valid || p.Nodes <= 0 || p.Root != 0 || !v11UndirectedConnected(p.Nodes, p.Edges) {
		return false
	}
	local := v11LocalNodes(state, root, 3)
	if len(local) < p.Nodes {
		return false
	}
	ids := make([]string, 0, len(local))
	out := map[string]map[string]bool{}
	in := map[string]map[string]bool{}
	for id := range local {
		ids = append(ids, id)
		out[id] = map[string]bool{}
		in[id] = map[string]bool{}
	}
	sort.Strings(ids)
	for _, e := range state.Edges {
		if local[e.From] && local[e.To] {
			out[e.From][e.To] = true
			in[e.To][e.From] = true
		}
	}
	patternOut := make([]int, p.Nodes)
	patternIn := make([]int, p.Nodes)
	need := make([][]int, p.Nodes)
	for _, e := range p.Edges {
		if e.From < 0 || e.From >= p.Nodes || e.To < 0 || e.To >= p.Nodes || e.From == e.To {
			return false
		}
		patternOut[e.From]++
		patternIn[e.To]++
		need[e.From] = append(need[e.From], e.To)
	}
	order := make([]int, 0, p.Nodes-1)
	for i := 1; i < p.Nodes; i++ {
		order = append(order, i)
	}
	sort.SliceStable(order, func(i, j int) bool {
		di := patternOut[order[i]] + patternIn[order[i]]
		dj := patternOut[order[j]] + patternIn[order[j]]
		if di != dj {
			return di > dj
		}
		return order[i] < order[j]
	})
	mapping := make([]string, p.Nodes)
	mapping[0] = root
	used := map[string]bool{root:true}

	compatible := func(pi int, gv string) bool {
		if len(out[gv]) < patternOut[pi] || len(in[gv]) < patternIn[pi] {
			return false
		}
		for _, pe := range p.Edges {
			if pe.From == pi && mapping[pe.To] != "" && !out[gv][mapping[pe.To]] {
				return false
			}
			if pe.To == pi && mapping[pe.From] != "" && !in[gv][mapping[pe.From]] {
				return false
			}
		}
		return true
	}

	var search func(int) bool
	search = func(pos int) bool {
		if pos == len(order) {
			return true
		}
		pi := order[pos]
		for _, gv := range ids {
			if used[gv] || !compatible(pi, gv) {
				continue
			}
			mapping[pi] = gv
			used[gv] = true
			if search(pos + 1) {
				return true
			}
			delete(used, gv)
			mapping[pi] = ""
		}
		return false
	}
	_ = need
	return search(0)
}

func (p V11DirectedExecutableRepresentation) Separates(examples []V8RelationalPatternExample) bool {
	seenPositive, seenNegative := false, false
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

func v11EncodePattern(nodes int, codes []int, pairs [][2]int) []V11DirectedPatternEdge {
	edges := make([]V11DirectedPatternEdge, 0, len(codes))
	for i, code := range codes {
		a, b := pairs[i][0], pairs[i][1]
		switch code {
		case 1:
			edges = append(edges, V11DirectedPatternEdge{From:a, To:b})
		case 2:
			edges = append(edges, V11DirectedPatternEdge{From:b, To:a})
		}
	}
	return edges
}

func v11EnumeratePatterns(maxNodes int) []V11DirectedExecutableRepresentation {
	if maxNodes < 2 {
		maxNodes = 2
	}
	if maxNodes > 4 {
		maxNodes = 4
	}
	out := make([]V11DirectedExecutableRepresentation, 0, 800)
	for nodes := 2; nodes <= maxNodes; nodes++ {
		pairs := make([][2]int, 0, nodes*(nodes-1)/2)
		for a := 0; a < nodes; a++ {
			for b := a + 1; b < nodes; b++ {
				pairs = append(pairs, [2]int{a,b})
			}
		}
		count := 1
		for i := 0; i < len(pairs); i++ {
			count *= 3
		}
		for code := 1; code < count; code++ {
			value := code
			codes := make([]int, len(pairs))
			for i := range codes {
				codes[i] = value % 3
				value /= 3
			}
			edges := v11EncodePattern(nodes, codes, pairs)
			if len(edges) == 0 || !v11UndirectedConnected(nodes, edges) {
				continue
			}
			out = append(out, V11DirectedExecutableRepresentation{
				Root:   0,
				Nodes:  nodes,
				Edges:  edges,
				Budget: 20000,
			})
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

func (r *V11DirectedExecutableRepresentation) Record(state RelationalState, action string, reward float64, terminal bool) {
	if r == nil || r.Retained {
		return
	}
	r.Examples = append(r.Examples, V8RelationalPatternExample{
		State: cloneRelationalState(state),
		Action: action,
		Success: reward > 0 || terminal,
	})
	if r.Valid && !r.Separates(r.Examples) {
		r.Valid = false
		r.Retained = false
		r.Edges = nil
	}
}

func (r *V11DirectedExecutableRepresentation) Synthesize() bool {
	if r == nil || r.Budget <= 0 || r.Retained {
		return r != nil && r.Retained && r.Valid
	}
	hasPositive, hasNegative := false, false
	for _, ex := range r.Examples {
		if ex.Success {
			hasPositive = true
		} else {
			hasNegative = true
		}
	}
	if !hasPositive || !hasNegative {
		return false
	}
	candidates := v11EnumeratePatterns(4)
	for _, candidate := range candidates {
		if r.SearchExpansions >= r.Budget {
			break
		}
		r.SearchExpansions++
		candidate.Examples = r.Examples
		candidate.Valid = candidate.Separates(r.Examples)
		if !candidate.Valid {
			continue
		}
		if r.Nodes == 0 || candidate.Complexity() < r.Complexity() || (candidate.Complexity() == r.Complexity() && candidate.Key() < r.Key()) {
			candidate.Budget = r.Budget
			candidate.SearchExpansions = r.SearchExpansions
			candidate.Examples = append([]V8RelationalPatternExample(nil), r.Examples...)
			r.Root = candidate.Root
			r.Nodes = candidate.Nodes
			r.Edges = append([]V11DirectedPatternEdge(nil), candidate.Edges...)
			r.Valid = true
			r.Retained = false
			r.Examples = candidate.Examples
		}
	}
	return r.Valid
}

func (r V11DirectedExecutableRepresentation) Select(state RelationalState, actions []string) (string, bool) {
	if !r.Valid || r.Nodes == 0 || len(r.Examples) > 0 && !r.Separates(r.Examples) {
		return "", false
	}
	matches := make([]string, 0, len(actions))
	for _, action := range actions {
		if r.Match(state, action) {
			matches = append(matches, action)
		}
	}
	if len(matches) == 0 {
		return "", false
	}
	sort.Strings(matches)
	return matches[0], true
}

func (r *V11DirectedExecutableRepresentation) ForgetExamples() error {
	if r == nil || !r.Valid || r.Nodes == 0 {
		return errors.New("no valid directed representation to retain")
	}
	r.Examples = nil
	r.Retained = true
	return nil
}

func (r V11DirectedExecutableRepresentation) Artifact() (string, error) {
	if !r.Valid || !r.Retained {
		return "", errors.New("directed representation is not retained")
	}
	artifact := V11DirectedPatternArtifact{
		Version:  1,
		Root:     r.Root,
		Nodes:    r.Nodes,
		Edges:    append([]V11DirectedPatternEdge(nil), r.Edges...),
	}
	data, err := json.Marshal(artifact)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func LoadV11DirectedRepresentation(artifact string) (V11DirectedExecutableRepresentation, error) {
	var a V11DirectedPatternArtifact
	if err := json.Unmarshal([]byte(artifact), &a); err != nil {
		return V11DirectedExecutableRepresentation{}, err
	}
	if a.Version != 1 || a.Root != 0 || a.Nodes < 2 || a.Nodes > 4 || len(a.Edges) == 0 || !v11UndirectedConnected(a.Nodes, a.Edges) {
		return V11DirectedExecutableRepresentation{}, errors.New("invalid directed representation artifact")
	}
	seen := map[string]bool{}
	for _, e := range a.Edges {
		if e.From < 0 || e.From >= a.Nodes || e.To < 0 || e.To >= a.Nodes || e.From == e.To {
			return V11DirectedExecutableRepresentation{}, errors.New("invalid directed representation edge")
		}
		key := fmt.Sprintf("%d>%d", e.From, e.To)
		if seen[key] {
			return V11DirectedExecutableRepresentation{}, errors.New("duplicate directed representation edge")
		}
		seen[key] = true
	}
	return V11DirectedExecutableRepresentation{
		Root:     a.Root,
		Nodes:    a.Nodes,
		Edges:    append([]V11DirectedPatternEdge(nil), a.Edges...),
		Budget:   20000,
		Valid:    true,
		Retained: true,
	}, nil
}
