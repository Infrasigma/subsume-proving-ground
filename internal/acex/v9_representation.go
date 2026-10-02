package acex

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type V9RepExample struct {
	State RelationalState
	Label bool
}

type V9RepAtom struct {
	Kind  string
	Value int
	Param string
}

func (a V9RepAtom) Key() string {
	if a.Param == "" {
		return fmt.Sprintf("%s:%d", a.Kind, a.Value)
	}
	return fmt.Sprintf("%s:%d:%s", a.Kind, a.Value, a.Param)
}

type V9RepExpr struct {
	Kind       string
	Atom       V9RepAtom
	Value      int
	Left, Right *V9RepExpr
}

func (e *V9RepExpr) Key() string {
	if e == nil {
		return "nil"
	}
	switch e.Kind {
	case "const":
		return "c:" + strconv.Itoa(e.Value)
	case "atom":
		return "a:" + e.Atom.Key()
	case "not", "mod2":
		return e.Kind + "(" + e.Left.Key() + ")"
	case "add", "sub", "eq", "lt", "and", "or":
		return e.Kind + "(" + e.Left.Key() + "," + e.Right.Key() + ")"
	default:
		return "unknown"
	}
}

func (e *V9RepExpr) Complexity() int {
	if e == nil {
		return 0
	}
	if e.Left == nil && e.Right == nil {
		return 1
	}
	return 1 + e.Left.Complexity() + e.Right.Complexity()
}

func (e *V9RepExpr) Eval(s RelationalState) (int, error) {
	if e == nil {
		return 0, errors.New("nil representation expression")
	}
	switch e.Kind {
	case "const":
		return e.Value, nil
	case "atom":
		return evalV9Atom(s, e.Atom), nil
	case "not":
		v, err := e.Left.Eval(s)
		if err != nil {
			return 0, err
		}
		if v == 0 {
			return 1, nil
		}
		return 0, nil
	case "mod2":
		v, err := e.Left.Eval(s)
		if err != nil {
			return 0, err
		}
		v %= 2
		if v < 0 {
			v += 2
		}
		return v, nil
	case "add", "sub", "eq", "lt", "and", "or":
		l, err := e.Left.Eval(s)
		if err != nil {
			return 0, err
		}
		r, err := e.Right.Eval(s)
		if err != nil {
			return 0, err
		}
		switch e.Kind {
		case "add":
			return l + r, nil
		case "sub":
			return l - r, nil
		case "eq":
			if l == r {
				return 1, nil
			}
			return 0, nil
		case "lt":
			if l < r {
				return 1, nil
			}
			return 0, nil
		case "and":
			if l != 0 && r != 0 {
				return 1, nil
			}
			return 0, nil
		case "or":
			if l != 0 || r != 0 {
				return 1, nil
			}
			return 0, nil
		}
	}
	return 0, fmt.Errorf("unsupported representation operator %q", e.Kind)
}

type V9Representation struct {
	Expr       *V9RepExpr
	Digest     string
	Complexity int
	TrainAcc   float64
}

func (r V9Representation) Apply(s RelationalState) (bool, error) {
	if r.Expr == nil {
		return false, errors.New("empty representation")
	}
	v, err := r.Expr.Eval(s)
	if err != nil {
		return false, err
	}
	return v != 0, nil
}

type V9RepresentationInventor struct {
	MaxDepth int
	Budget   int
	MinTrainAccuracy float64
}

func (v V9RepresentationInventor) Invent(train []V9RepExample) (V9Representation, error) {
	if len(train) < 4 {
		return V9Representation{}, errors.New("representation invention needs at least four examples")
	}
	maxDepth := v.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}
	budget := v.Budget
	if budget <= 0 {
		budget = 20000
	}
	target := v.MinTrainAccuracy
	if target <= 0 {
		target = 1
	}

	atoms := v9CollectAtoms(train)
	exprs := make(map[string]*V9RepExpr)
	frontier := make([]*V9RepExpr, 0, len(atoms)+3)
	for _, atom := range atoms {
		e := &V9RepExpr{Kind:"atom", Atom:atom}
		exprs[e.Key()] = e
		frontier = append(frontier, e)
	}
	for _, n := range []int{0, 1} {
		e := &V9RepExpr{Kind:"const", Value:n}
		exprs[e.Key()] = e
		frontier = append(frontier, e)
	}

	best, bestAcc := chooseV9Best(train, frontier, nil)
	if best != nil && bestAcc >= target {
		return finishV9Representation(best, bestAcc), nil
	}

	used := len(frontier)
	for depth := 1; depth <= maxDepth && used < budget; depth++ {
		next := make([]*V9RepExpr, 0, minV9Int(len(frontier)*4, budget-used))
		for _, a := range frontier {
			if used >= budget {
				break
			}
			for _, op := range []string{"not", "mod2"} {
				cand := &V9RepExpr{Kind:op, Left:a}
				key := cand.Key()
				if _, ok := exprs[key]; ok {
					continue
				}
				exprs[key] = cand
				next = append(next, cand)
				used++
				if acc := v9Accuracy(train, cand); acc >= target {
					return finishV9Representation(cand, acc), nil
				}
				if used >= budget {
					break
				}
			}
		}
		for i := 0; i < len(frontier) && used < budget; i++ {
			for j := i; j < len(frontier) && used < budget; j++ {
				a, b := frontier[i], frontier[j]
				for _, op := range []string{"add", "sub", "eq", "lt", "and", "or"} {
					cand := canonicalV9Binary(op, a, b)
					key := cand.Key()
					if _, ok := exprs[key]; ok {
						continue
					}
					exprs[key] = cand
					next = append(next, cand)
					used++
					if acc := v9Accuracy(train, cand); acc >= target {
						return finishV9Representation(cand, acc), nil
					}
				}
			}
		}
		if len(next) == 0 {
			break
		}
		frontier = retainV9Diverse(next, train, 256)
		best, bestAcc = chooseV9Best(train, frontier, best)
		if best != nil && bestAcc >= target {
			return finishV9Representation(best, bestAcc), nil
		}
	}
	if best == nil || bestAcc < target {
		return V9Representation{}, fmt.Errorf("representation search exhausted: best_train_accuracy=%.3f budget=%d", bestAcc, budget)
	}
	return finishV9Representation(best, bestAcc), nil
}

func finishV9Representation(e *V9RepExpr, acc float64) V9Representation {
	return V9Representation{
		Expr:e,
		Digest:hashString(e.Key()),
		Complexity:e.Complexity(),
		TrainAcc:acc,
	}
}

func canonicalV9Binary(op string, a, b *V9RepExpr) *V9RepExpr {
	if op == "eq" || op == "and" || op == "or" || op == "add" {
		if b.Key() < a.Key() {
			a, b = b, a
		}
	}
	return &V9RepExpr{Kind:op, Left:a, Right:b}
}

func v9Accuracy(examples []V9RepExample, e *V9RepExpr) float64 {
	if len(examples) == 0 {
		return 0
	}
	ok := 0
	for _, ex := range examples {
		v, err := e.Eval(ex.State)
		if err == nil && (v != 0) == ex.Label {
			ok++
		}
	}
	return float64(ok) / float64(len(examples))
}

func chooseV9Best(examples []V9RepExample, candidates []*V9RepExpr, incumbent *V9RepExpr) (*V9RepExpr, float64) {
	best := incumbent
	bestAcc := 0.0
	if incumbent != nil {
		bestAcc = v9Accuracy(examples, incumbent)
	}
	for _, e := range candidates {
		acc := v9Accuracy(examples, e)
		if acc > bestAcc || (acc == bestAcc && acc == 1 && (best == nil || e.Complexity() < best.Complexity())) {
			best, bestAcc = e, acc
		}
	}
	return best, bestAcc
}

func retainV9Diverse(candidates []*V9RepExpr, examples []V9RepExample, limit int) []*V9RepExpr {
	if len(candidates) <= limit {
		return candidates
	}
	type scored struct {
		e   *V9RepExpr
		acc float64
	}
	s := make([]scored, 0, len(candidates))
	for _, e := range candidates {
		acc := v9Accuracy(examples, e)
		if acc >= 0.5 {
			s = append(s, scored{e:e, acc:acc})
		}
	}
	sort.SliceStable(s, func(i, j int) bool {
		if s[i].acc != s[j].acc {
			return s[i].acc > s[j].acc
		}
		if s[i].e.Complexity() != s[j].e.Complexity() {
			return s[i].e.Complexity() < s[j].e.Complexity()
		}
		return s[i].e.Key() < s[j].e.Key()
	})
	if len(s) > limit {
		s = s[:limit]
	}
	out := make([]*V9RepExpr, 0, len(s))
	for _, x := range s {
		out = append(out, x.e)
	}
	return out
}

func v9CollectAtoms(train []V9RepExample) []V9RepAtom {
	set := map[string]V9RepAtom{}
	for _, ex := range train {
		maxDegree := len(ex.State.Nodes)
		for _, n := range ex.State.Nodes {
			set[V9RepAtom{Kind:"node-count"}.Key()] = V9RepAtom{Kind:"node-count"}
			_ = n
		}
		set[V9RepAtom{Kind:"edge-count"}.Key()] = V9RepAtom{Kind:"edge-count"}
		for d := 0; d <= maxDegree; d++ {
			set[V9RepAtom{Kind:"degree-class-count",Value:d}.Key()] = V9RepAtom{Kind:"degree-class-count",Value:d}
		}
	}
	out := make([]V9RepAtom, 0, len(set))
	for _, a := range set {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

func evalV9Atom(s RelationalState, a V9RepAtom) int {
	switch a.Kind {
	case "node-count":
		return len(s.Nodes)
	case "edge-count":
		return len(s.Edges)
	case "degree-class-count":
		degree := map[string]int{}
		for _, n := range s.Nodes {
			degree[n.ID] = 0
		}
		for _, e := range s.Edges {
			degree[e.From]++
			degree[e.To]++
		}
		count := 0
		for _, d := range degree {
			if d == a.Value {
				count++
			}
		}
		return count
	default:
		return 0
	}
}

type V9RepresentationLibrary struct {
	Items []V9Representation
}

func (l *V9RepresentationLibrary) Admit(r V9Representation, independentHoldout []V9RepExample) error {
	if r.Expr == nil || len(independentHoldout) == 0 {
		return errors.New("representation admission requires artifact and independent holdout")
	}
	if v9Accuracy(independentHoldout, r.Expr) < 0.90 {
		return errors.New("representation rejected by independent holdout")
	}
	for _, existing := range l.Items {
		if existing.Digest == r.Digest {
			return nil
		}
	}
	l.Items = append(l.Items, r)
	return nil
}

func (l V9RepresentationLibrary) ApplyAll(s RelationalState) []bool {
	out := make([]bool, 0, len(l.Items))
	for _, r := range l.Items {
		v, err := r.Apply(s)
		out = append(out, err == nil && v)
	}
	return out
}

func V9RepresentationDescription(r V9Representation) string {
	if r.Expr == nil {
		return ""
	}
	var rec func(*V9RepExpr) string
	rec = func(e *V9RepExpr) string {
		if e == nil {
			return ""
		}
		switch e.Kind {
		case "const":
			return strconv.Itoa(e.Value)
		case "atom":
			return e.Atom.Key()
		case "not", "mod2":
			return e.Kind + "(" + rec(e.Left) + ")"
		default:
			return "(" + rec(e.Left) + " " + e.Kind + " " + rec(e.Right) + ")"
		}
	}
	return strings.ReplaceAll(rec(r.Expr), " ", " ")
}

func minV9Int(a, b int) int {
	if a < b {
		return a
	}
	return b
}
