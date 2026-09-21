package acex

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type V8ExecutableRepFeature int

const (
	V8RepOutEdges V8ExecutableRepFeature = iota
	V8RepInEdges
	V8RepIncidentEdges
	V8RepNodeCount
	V8RepEdgeCount
)

type V8ExecutableRepAtom struct {
	Feature V8ExecutableRepFeature
	Const   int
	UseConst bool
}

type V8ExecutableRepPredicate struct {
	Left       V8ExecutableRepAtom
	Op         string
	Right      V8ExecutableRepAtom
	RightConst int
}

type V8ExecutableRepresentation struct {
	Predicate   V8ExecutableRepPredicate
	Examples    []V8RelationalPatternExample
	SearchSpace int
	ProgramKey  string
	Valid       bool
}

func NewV8ExecutableRepresentation() V8ExecutableRepresentation {
	return V8ExecutableRepresentation{SearchSpace: 0}
}

func v8RepFeatureName(f V8ExecutableRepFeature) string {
	switch f {
	case V8RepOutEdges:
		return "count(outgoing-root-incidence)"
	case V8RepInEdges:
		return "count(incoming-root-incidence)"
	case V8RepIncidentEdges:
		return "count(incident-root-edges)"
	case V8RepNodeCount:
		return "count(nodes)"
	case V8RepEdgeCount:
		return "count(edges)"
	default:
		return "unknown-feature"
	}
}

func v8RepFeatureValue(g RelationalState, root string, f V8ExecutableRepFeature) (int, bool) {
	present := false
	for _, n := range g.Nodes {
		if n.ID == root {
			present = true
			break
		}
	}
	if !present {
		return 0, false
	}
	switch f {
	case V8RepOutEdges:
		n := 0
		for _, e := range g.Edges {
			if e.From == root {
				n++
			}
		}
		return n, true
	case V8RepInEdges:
		n := 0
		for _, e := range g.Edges {
			if e.To == root {
				n++
			}
		}
		return n, true
	case V8RepIncidentEdges:
		n := 0
		for _, e := range g.Edges {
			if e.From == root || e.To == root {
				n++
			}
		}
		return n, true
	case V8RepNodeCount:
		return len(g.Nodes), true
	case V8RepEdgeCount:
		return len(g.Edges), true
	default:
		return 0, false
	}
}

func v8RepAtomValue(g RelationalState, root string, a V8ExecutableRepAtom) (int, bool) {
	if a.UseConst {
		return a.Const, true
	}
	return v8RepFeatureValue(g, root, a.Feature)
}

func (p V8ExecutableRepPredicate) Match(g RelationalState, root string) bool {
	lv, lok := v8RepAtomValue(g, root, p.Left)
	if !lok {
		return false
	}
	rv := p.RightConst
	if !p.Right.UseConst {
		var ok bool
		rv, ok = v8RepAtomValue(g, root, p.Right)
		if !ok {
			return false
		}
	}
	switch p.Op {
	case "eq":
		return lv == rv
	case "neq":
		return lv != rv
	case "gt":
		return lv > rv
	case "ge":
		return lv >= rv
	case "lt":
		return lv < rv
	case "le":
		return lv <= rv
	default:
		return false
	}
}

func (p V8ExecutableRepPredicate) Key() string {
	atom := func(a V8ExecutableRepAtom) string {
		if a.UseConst {
			return "const:" + strconv.Itoa(a.Const)
		}
		return v8RepFeatureName(a.Feature)
	}
	if p.Right.UseConst {
		return atom(p.Left) + "|" + p.Op + "|const:" + strconv.Itoa(p.RightConst)
	}
	return atom(p.Left) + "|" + p.Op + "|" + atom(p.Right)
}

func (r *V8ExecutableRepresentation) Record(state RelationalState, action string, reward float64, terminal bool) {
	if r == nil {
		return
	}
	r.Examples = append(r.Examples, V8RelationalPatternExample{
		State: cloneRelationalState(state),
		Action: action,
		Success: reward > 0 || terminal,
	})
	if r.ProgramKey != "" && !r.separates() {
		r.Valid = false
		r.ProgramKey = ""
	}
}

func (r V8ExecutableRepresentation) separates() bool {
	if r.ProgramKey == "" || len(r.Examples) == 0 {
		return false
	}
	seenPos, seenNeg := false, false
	for _, ex := range r.Examples {
		matched := r.Predicate.Match(ex.State, ex.Action)
		if ex.Success {
			seenPos = true
			if !matched {
				return false
			}
		} else {
			seenNeg = true
			if matched {
				return false
			}
		}
	}
	return seenPos && seenNeg
}

func (r *V8ExecutableRepresentation) Synthesize() bool {
	if r == nil || len(r.Examples) < 2 {
		return false
	}
	if r.separates() {
		r.Valid = true
		return true
	}
	features := []V8ExecutableRepFeature{
		V8RepOutEdges,
		V8RepInEdges,
		V8RepIncidentEdges,
		V8RepNodeCount,
		V8RepEdgeCount,
	}
	ops := []string{"eq", "neq", "gt", "ge", "lt", "le"}
	best := V8ExecutableRepPredicate{}
	bestKey := ""
	bestComplexity := int(^uint(0) >> 1)
	expansions := 0

	// Generic relational-program search: it composes state-derived scalar
	// measurements and comparisons. No target relation or task labels are
	// encoded here.
	for _, lf := range features {
		for _, op := range ops {
			for c := -1; c <= 8; c++ {
				expansions++
				p := V8ExecutableRepPredicate{
					Left: V8ExecutableRepAtom{Feature: lf},
					Op: op,
					Right: V8ExecutableRepAtom{UseConst: true, Const: c},
					RightConst: c,
				}
				r.Predicate = p
				if !r.separates() {
					continue
				}
				key := p.Key()
				if bestKey == "" || key < bestKey {
					best = p
					bestKey = key
					bestComplexity = 2
				}
			}
		}
	}

	for _, lf := range features {
		for _, rf := range features {
			if lf == rf {
				continue
			}
			for _, op := range ops {
				expansions++
				p := V8ExecutableRepPredicate{
					Left: V8ExecutableRepAtom{Feature: lf},
					Op: op,
					Right: V8ExecutableRepAtom{Feature: rf},
				}
				r.Predicate = p
				if !r.separates() {
					continue
				}
				key := p.Key()
				if bestKey == "" || bestComplexity > 3 || (bestComplexity == 3 && key < bestKey) {
					best = p
					bestKey = key
					bestComplexity = 3
				}
			}
		}
	}

	r.SearchSpace += expansions
	if bestKey == "" {
		r.Valid = false
		r.ProgramKey = ""
		return false
	}
	r.Predicate = best
	r.ProgramKey = bestKey
	r.Valid = true
	return true
}

func (r V8ExecutableRepresentation) Select(state RelationalState, actions []string) (string, bool) {
	if !r.Valid || r.ProgramKey == "" {
		return "", false
	}
	// After raw-episode deletion the retained executable program remains
	// admissible; when examples are still resident, they must continue to
	// separate positive from negative evidence.
	if len(r.Examples) > 0 && !r.separates() {
		return "", false
	}
	matches := make([]string, 0, len(actions))
	for _, action := range actions {
		if r.Predicate.Match(state, action) {
			matches = append(matches, action)
		}
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

func (r V8ExecutableRepresentation) ForgetExamples() V8ExecutableRepresentation {
	r.Examples = nil
	return r
}

func (r V8ExecutableRepresentation) Description() string {
	if r.ProgramKey == "" {
		return "unlearned"
	}
	return fmt.Sprintf("predicate{%s}", strings.ReplaceAll(r.ProgramKey, "|", " "))
}

func (r V8ExecutableRepresentation) Validate() error {
	if r.ProgramKey == "" || !r.Valid {
		return errors.New("executable representation not valid")
	}
	if len(r.Examples) > 0 && !r.separates() {
		return errors.New("executable representation does not separate retained evidence")
	}
	return nil
}
