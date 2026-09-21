package acex

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type V3Type string

const V3Int V3Type = "Int"

type V3Expr struct {
	Kind  string
	Value int
	A     *V3Expr
	B     *V3Expr
	Macro string
}

type V3Task struct {
	ID          string
	TrainInputs []int
	TrainOutput []int
	HoldInputs  []int
	HoldOutput  []int
}

type V3Macro struct {
	Name       string
	Body       V3Expr
	ArgType    V3Type
	ReturnType V3Type
	DefSize    int
	UseCount   int
	Savings    int
	Parent     string
	Provenance []string
}

type V3Library struct {
	Version uint64
	Macros  map[string]V3Macro
}

type V3SearchResult struct {
	Program V3Expr
	Cost    Resource
	Size    int
	Found   bool
}

type V3LearnResult struct {
	Library     V3Library
	BeforeCost  int
	AfterCost   int
	Added       []V3Macro
	SearchTrace []string
}

type V3HiddenCase struct {
	ID      string
	Train   V3Task
	Holdout V3Task
}

func NewV3Library() V3Library {
	return V3Library{Macros: map[string]V3Macro{}}
}

func cloneV3Expr(e V3Expr) V3Expr {
	out := e
	if e.A != nil {
		a := cloneV3Expr(*e.A)
		out.A = &a
	}
	if e.B != nil {
		b := cloneV3Expr(*e.B)
		out.B = &b
	}
	return out
}

func cloneV3Library(in V3Library) V3Library {
	out := NewV3Library()
	out.Version = in.Version
	for k, v := range in.Macros {
		vv := v
		vv.Body = cloneV3Expr(v.Body)
		vv.Provenance = append([]string(nil), v.Provenance...)
		out.Macros[k] = vv
	}
	return out
}

func v3Size(e V3Expr) int {
	if e.Kind == "" {
		return 0
	}
	n := 1
	if e.A != nil {
		n += v3Size(*e.A)
	}
	if e.B != nil {
		n += v3Size(*e.B)
	}
	return n
}

func v3Signature(e V3Expr) string {
	switch e.Kind {
	case "input":
		return "x"
	case "const":
		return "c:" + strconv.Itoa(e.Value)
	case "macro":
		return "m:" + e.Macro
	case "neg", "abs":
		return e.Kind + "(" + v3Signature(*e.A) + ")"
	case "add", "mul", "max", "min":
		return e.Kind + "(" + v3Signature(*e.A) + "," + v3Signature(*e.B) + ")"
	default:
		return "?"
	}
}

func (l V3Library) Signature() string {
	names := make([]string, 0, len(l.Macros))
	for name := range l.Macros {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		m := l.Macros[name]
		parts = append(parts, fmt.Sprintf("%s=%s", name, v3Signature(m.Body)))
	}
	return strings.Join(parts, "|")
}

func (l V3Library) Digest() string {
	return hashString(l.Signature())
}

func evalV3Expr(e V3Expr, x int, l V3Library, stack map[string]bool) (int, error) {
	switch e.Kind {
	case "input":
		return x, nil
	case "const":
		return e.Value, nil
	case "neg":
		v, err := evalV3Expr(*e.A, x, l, stack)
		return -v, err
	case "abs":
		v, err := evalV3Expr(*e.A, x, l, stack)
		if err != nil {
			return 0, err
		}
		return int(math.Abs(float64(v))), nil
	case "add", "mul", "max", "min":
		a, err := evalV3Expr(*e.A, x, l, stack)
		if err != nil {
			return 0, err
		}
		b, err := evalV3Expr(*e.B, x, l, stack)
		if err != nil {
			return 0, err
		}
		switch e.Kind {
		case "add":
			return a + b, nil
		case "mul":
			return a * b, nil
		case "max":
			if a > b {
				return a, nil
			}
			return b, nil
		default:
			if a < b {
				return a, nil
			}
			return b, nil
		}
	case "macro":
		if stack[e.Macro] {
			return 0, errors.New("recursive library cycle")
		}
		m, ok := l.Macros[e.Macro]
		if !ok {
			return 0, fmt.Errorf("unknown macro %q", e.Macro)
		}
		stack[e.Macro] = true
		v, err := evalV3Expr(m.Body, x, l, stack)
		delete(stack, e.Macro)
		return v, err
	default:
		return 0, fmt.Errorf("unknown V3 expr kind %q", e.Kind)
	}
}

func v3Evaluate(e V3Expr, inputs []int, l V3Library) ([]int, error) {
	out := make([]int, len(inputs))
	for i, x := range inputs {
		v, err := evalV3Expr(e, x, l, map[string]bool{})
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func v3VectorKey(xs []int) string {
	var b strings.Builder
	for i, x := range xs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(x))
	}
	return b.String()
}

func v3MatchesTrain(e V3Expr, task V3Task, l V3Library) bool {
	got, err := v3Evaluate(e, task.TrainInputs, l)
	if err != nil || len(got) != len(task.TrainOutput) {
		return false
	}
	for i := range got {
		if got[i] != task.TrainOutput[i] {
			return false
		}
	}
	return true
}

func v3VerifyHoldout(e V3Expr, task V3Task, l V3Library) bool {
	got, err := v3Evaluate(e, task.HoldInputs, l)
	if err != nil || len(got) != len(task.HoldOutput) {
		return false
	}
	for i := range got {
		if got[i] != task.HoldOutput[i] {
			return false
		}
	}
	return true
}

func v3PrimitiveExprs(l V3Library) []V3Expr {
	out := []V3Expr{{Kind: "input"}}
	for c := -3; c <= 3; c++ {
		out = append(out, V3Expr{Kind: "const", Value: c})
	}
	names := make([]string, 0, len(l.Macros))
	for name := range l.Macros {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out = append(out, V3Expr{Kind: "macro", Macro: name})
	}
	return out
}

func v3SemanticExpand(task V3Task, l V3Library, maxSize int, beam int) (V3SearchResult, error) {
	if maxSize < 1 || beam < 1 {
		return V3SearchResult{}, errors.New("invalid V3 search bounds")
	}
	type entry struct {
		e    V3Expr
		key  string
		size int
	}
	bySize := map[int][]entry{}
	seen := map[string]bool{}
	result := V3SearchResult{}
	for _, e := range v3PrimitiveExprs(l) {
		key, err := v3Evaluate(e, task.TrainInputs, l)
		if err != nil {
			continue
		}
		k := v3VectorKey(key)
		if !seen[k] {
			seen[k] = true
			bySize[1] = append(bySize[1], entry{e: e, key: k, size: 1})
			result.Cost.Search += 1
		}
	}
	sizes := map[int]bool{1: true}
	for size := 1; size <= maxSize; size++ {
		entries := bySize[size]
		sort.Slice(entries, func(i, j int) bool { return v3Signature(entries[i].e) < v3Signature(entries[j].e) })
		if len(entries) > beam {
			entries = entries[:beam]
		}
		bySize[size] = entries
		for _, en := range entries {
			if v3MatchesTrain(en.e, task, l) {
				result.Cost.Verify += len(task.TrainInputs) + len(task.HoldInputs)
				result.Program = cloneV3Expr(en.e)
				result.Size = en.size
				result.Found = true
				return result, nil
			}
		}
		if size == maxSize {
			break
		}
		for nextSize := size + 1; nextSize <= maxSize; nextSize++ {
			sizes[nextSize] = true
		}
		// Unary expansions.
		for _, en := range entries {
			for _, kind := range []string{"neg", "abs"} {
				e := V3Expr{Kind: kind, A: &en.e}
				vals, err := v3Evaluate(e, task.TrainInputs, l)
				if err != nil {
					continue
				}
				k := v3VectorKey(vals)
				if !seen[k] {
					seen[k] = true
					bySize[size+1] = append(bySize[size+1], entry{e: e, key: k, size: size + 1})
					result.Cost.Search++
				}
			}
		}
		// Binary expansions from the bounded semantic beam.
		for leftSize := 1; leftSize <= size; leftSize++ {
			rightSize := size + 1 - 1 - leftSize
			if rightSize < 1 || rightSize > size {
				continue
			}
			for _, a := range bySize[leftSize] {
				for _, b := range bySize[rightSize] {
					for _, kind := range []string{"add", "mul", "max", "min"} {
						e := V3Expr{Kind: kind, A: &a.e, B: &b.e}
						vals, err := v3Evaluate(e, task.TrainInputs, l)
						if err != nil {
							continue
						}
						k := v3VectorKey(vals)
						if !seen[k] {
							seen[k] = true
							bySize[size+1] = append(bySize[size+1], entry{e: e, key: k, size: size + 1})
							result.Cost.Search++
						}
					}
				}
			}
		}
	}
	return result, errors.New("V3 search exhausted without verified program")
}

func v3Subexprs(e V3Expr, out map[string]V3Expr) {
	if e.Kind == "" {
		return
	}
	sz := v3Size(e)
	if sz >= 2 && e.Kind != "input" && e.Kind != "const" && e.Kind != "macro" {
		out[v3Signature(e)] = cloneV3Expr(e)
	}
	if e.A != nil {
		v3Subexprs(*e.A, out)
	}
	if e.B != nil {
		v3Subexprs(*e.B, out)
	}
}

func v3CountOccurrence(e V3Expr, sig string) int {
	n := 0
	if v3Signature(e) == sig {
		n++
	}
	if e.A != nil {
		n += v3CountOccurrence(*e.A, sig)
	}
	if e.B != nil {
		n += v3CountOccurrence(*e.B, sig)
	}
	return n
}

func v3ContainsMacro(e V3Expr, name string) bool {
	if e.Kind == "macro" && e.Macro == name {
		return true
	}
	if e.A != nil && v3ContainsMacro(*e.A, name) {
		return true
	}
	if e.B != nil && v3ContainsMacro(*e.B, name) {
		return true
	}
	return false
}

func v3ReplaceExpr(e V3Expr, target string, macro V3Expr) (V3Expr, bool) {
	if v3Signature(e) == target && v3Size(e) > 1 {
		return cloneV3Expr(macro), true
	}
	out := cloneV3Expr(e)
	changed := false
	if out.A != nil {
		a, ok := v3ReplaceExpr(*out.A, target, macro)
		out.A = &a
		changed = changed || ok
	}
	if out.B != nil {
		b, ok := v3ReplaceExpr(*out.B, target, macro)
		out.B = &b
		changed = changed || ok
	}
	return out, changed
}

func v3BestCandidate(taskPrograms map[string]V3Expr, tasks []V3Task, l V3Library) (V3Macro, bool) {
	type stat struct {
		expr  V3Expr
		uses  int
		sizes int
		tasks map[string]bool
	}
	stats := map[string]*stat{}
	for _, task := range tasks {
		p, ok := taskPrograms[task.ID]
		if !ok {
			continue
		}
		sub := map[string]V3Expr{}
		v3Subexprs(p, sub)
		for sig, e := range sub {
			st := stats[sig]
			if st == nil {
				st = &stat{expr: e, tasks: map[string]bool{}}
				stats[sig] = st
			}
			st.uses += v3CountOccurrence(p, sig)
			st.sizes = v3Size(e)
			st.tasks[task.ID] = true
		}
	}
	var names []string
	for sig, st := range stats {
		if len(st.tasks) < 2 || st.sizes < 2 {
			continue
		}
		// A library definition costs its body once; a macro call costs one node.
		savings := st.uses * (st.sizes - 1)
		if savings <= st.sizes {
			continue
		}
		names = append(names, sig)
	}
	sort.Strings(names)
	sort.Slice(names, func(i, j int) bool {
		if stats[names[i]].sizes != stats[names[j]].sizes {
			return stats[names[i]].sizes > stats[names[j]].sizes
		}
		if stats[names[i]].uses != stats[names[j]].uses {
			return stats[names[i]].uses > stats[names[j]].uses
		}
		return names[i] < names[j]
	})
	for _, sig := range names {
		st := stats[sig]
		body := cloneV3Expr(st.expr)
		return V3Macro{
			Name:       fmt.Sprintf("v3skill-%d", len(l.Macros)+1),
			Body:       body,
			ArgType:    V3Int,
			ReturnType: V3Int,
			DefSize:    v3Size(body),
			UseCount:   st.uses,
			Savings:    st.uses * (st.sizes - 1),
			Parent:     "base",
			Provenance: append([]string(nil), names...),
		}, true
	}
	return V3Macro{}, false
}

func V3LearnGeneration(tasks []V3Task, base V3Library, maxSize, beam int) (V3Library, int, int, []V3SearchResult, error) {
	if len(tasks) < 2 {
		return base, 0, 0, nil, errors.New("need at least two tasks for library induction")
	}
	before := 0
	programs := map[string]V3Expr{}
	results := make([]V3SearchResult, 0, len(tasks))
	for _, task := range tasks {
		res, err := v3SemanticExpand(task, base, maxSize, beam)
		if err != nil {
			return base, before, before, results, err
		}
		if !v3VerifyHoldout(res.Program, task, base) {
			return base, before, before, results, fmt.Errorf("visible task %s failed independent holdout program=%s size=%d", task.ID, v3Signature(res.Program), res.Size)
		}
		before += res.Cost.Total()
		programs[task.ID] = cloneV3Expr(res.Program)
		results = append(results, res)
	}
	macro, ok := v3BestCandidate(programs, tasks, base)
	if !ok {
		return base, before, before, results, errors.New("no reusable cross-task abstraction found")
	}
	afterLib := cloneV3Library(base)
	afterLib.Macros[macro.Name] = macro
	after := 0
	for _, task := range tasks {
		res, err := v3SemanticExpand(task, afterLib, maxSize, beam)
		if err != nil {
			return base, before, before, results, err
		}
		if !v3VerifyHoldout(res.Program, task, afterLib) {
			return base, before, after, results, fmt.Errorf("admitted library fails holdout on visible task %s", task.ID)
		}
		after += res.Cost.Total()
	}
	if after >= before {
		return base, before, after, results, fmt.Errorf("candidate library did not reduce verified acquisition cost: before=%d after=%d", before, after)
	}
	afterLib.Version++
	return afterLib, before, after, results, nil
}

func V3LearnTwoGenerations(first, second []V3Task, seedLib V3Library, maxSize, beam int) (V3LearnResult, error) {
	l1, b1, a1, _, err := V3LearnGeneration(first, seedLib, maxSize, beam)
	if err != nil {
		return V3LearnResult{Library: seedLib, BeforeCost: b1, AfterCost: a1}, err
	}
	l1m := l1.Macros[fmt.Sprintf("v3skill-%d", len(seedLib.Macros)+1)]
	l2, b2, a2, _, err := V3LearnGeneration(second, l1, maxSize, beam)
	if err != nil {
		return V3LearnResult{Library: l1, BeforeCost: b1 + b2, AfterCost: a1 + a2, Added: []V3Macro{l1m}}, err
	}
	l2m := l2.Macros[fmt.Sprintf("v3skill-%d", len(l1.Macros))]
	if !v3ContainsMacro(l2m.Body, l1m.Name) {
		return V3LearnResult{Library: l2, BeforeCost: b1 + b2, AfterCost: a1 + a2, Added: []V3Macro{l1m, l2m}, SearchTrace: []string{"second-generation abstraction does not reference first-generation macro"}}, errors.New("recursive library composition was not demonstrated")
	}
	return V3LearnResult{
		Library: l2,
		BeforeCost: b1 + b2,
		AfterCost: a1 + a2,
		Added: []V3Macro{l1m, l2m},
		SearchTrace: append([]string{"generation-1="+l1m.Name}, fmt.Sprintf("generation-2=%s parent=%s", l2m.Name, l1m.Name)),
	}, nil
}

func V3VerifyHiddenSuccessors(lib V3Library, baseline V3Library, hidden []V3HiddenCase, maxSize, beam int) ([]float64, int, int, error) {
	if len(hidden) < 3 {
		return nil, 0, 0, errors.New("need at least three hidden successor tasks")
	}
	ratios := make([]float64, 0, len(hidden))
	before, after := 0, 0
	for _, hc := range hidden {
		base, baseErr := v3SemanticExpand(hc.Train, baseline, maxSize, beam)
		if baseErr != nil {
			return nil, before, after, fmt.Errorf("baseline hidden search failed %s: %w", hc.ID, baseErr)
		}
		got, err := v3SemanticExpand(hc.Train, lib, maxSize, beam)
		if err != nil {
			return nil, before, after, fmt.Errorf("learned hidden search failed %s: %w", hc.ID, err)
		}
		if !v3MatchesTrain(got.Program, hc.Train, lib) || !v3VerifyHoldout(got.Program, hc.Train, lib) || !v3VerifyHoldout(got.Program, hc.Holdout, lib) {
			return nil, before, after, fmt.Errorf("hidden semantic generalization failed %s", hc.ID)
		}
		before += base.Cost.Total()
		after += got.Cost.Total()
		r := float64(got.Cost.Total()) / math.Max(float64(base.Cost.Total()), 1)
		ratios = append(ratios, r)
		if r >= 0.80 {
			return ratios, before, after, fmt.Errorf("hidden cost ratio %.3f >= 0.80 for %s", r, hc.ID)
		}
	}
	return ratios, before, after, nil
}

func V3TaskFromExpr(id string, expr V3Expr, xs, hold []int, l V3Library) (V3Task, error) {
	y, err := v3Evaluate(expr, xs, l)
	if err != nil {
		return V3Task{}, err
	}
	h, err := v3Evaluate(expr, hold, l)
	if err != nil {
		return V3Task{}, err
	}
	return V3Task{ID: id, TrainInputs: append([]int(nil), xs...), TrainOutput: y, HoldInputs: append([]int(nil), hold...), HoldOutput: h}, nil
}

func V3MacroCall(name string) V3Expr {
	return V3Expr{Kind: "macro", Macro: name}
}

func V3Unary(kind string, a V3Expr) V3Expr {
	return V3Expr{Kind: kind, A: &a}
}

func V3Binary(kind string, a, b V3Expr) V3Expr {
	return V3Expr{Kind: kind, A: &a, B: &b}
}

func V3Input() V3Expr {
	return V3Expr{Kind: "input"}
}

func V3Const(v int) V3Expr {
	return V3Expr{Kind: "const", Value: v}
}

func V3MakeGenerationOneTasks(lib V3Library, seed int64) ([]V3Task, error) {
	// The seed changes the latent concept itself while preserving the same
	// type and grammar complexity. This makes evaluator-runtime seeding
	// scientifically meaningful without changing the acceptance rule.
	offset := int(seed%5) - 2
	macroSeed := V3Unary("abs", V3Binary("add", V3Input(), V3Const(offset)))
	tasks := make([]V3Task, 0, 3)
	defs := []V3Expr{
		V3Binary("add", macroSeed, V3Const(2)),
		V3Binary("mul", V3Const(2), macroSeed),
		V3Binary("max", macroSeed, V3Const(3)),
	}
	xs := []int{-4, -3, -1, 0, 2, 4}
	hold := []int{-5, -2, 1, 3, 5}
	for i, def := range defs {
		t, err := V3TaskFromExpr(fmt.Sprintf("g1-%d", i), def, xs, hold, lib)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func V3MakeGenerationTwoTasks(lib V3Library, macroName string) ([]V3Task, error) {
	m := V3MacroCall(macroName)
	reuse := V3Binary("add", m, m)
	defs := []V3Expr{
		V3Binary("add", reuse, V3Const(1)),
		V3Binary("mul", V3Const(2), reuse),
		V3Binary("max", reuse, V3Const(5)),
	}
	xs := []int{-6, -4, -2, 0, 2, 5}
	hold := []int{-7, -3, 1, 3, 6}
	tasks := make([]V3Task, 0, len(defs))
	for i, def := range defs {
		t, err := V3TaskFromExpr(fmt.Sprintf("g2-%d", i), def, xs, hold, lib)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func V3MakeHiddenSuccessors(lib V3Library, first, second string, seed int64) ([]V3HiddenCase, error) {
	_ = first
	seedExpr := V3MacroCall(second)
	// The evaluator composes the learned second-generation abstraction with
	// fresh constants and fresh input/holdout draws. The learner sees only
	// input/output pairs, never this construction recipe.
	offset := int((seed % 5) - 2)
	targets := []V3Expr{
		V3Binary("add", seedExpr, V3Const(offset)),
		V3Binary("max", seedExpr, V3Const(4)),
		V3Binary("mul", seedExpr, V3Const(2)),
		V3Binary("add", seedExpr, V3Const(-2)),
	}
	trains := [][]int{{-9, -5, -2, 1, 4, 7}, {-8, -4, -1, 2, 5, 9}, {-10, -6, -3, 0, 3, 8}, {-7, -5, -1, 1, 6, 10}}
	holds := [][]int{{-11, -6, -3, 0, 3, 6, 9}, {-10, -5, 0, 3, 7, 10}, {-12, -7, -4, 1, 4, 9}, {-9, -4, 0, 2, 7, 11}}
	out := make([]V3HiddenCase, 0, len(targets))
	for i, target := range targets {
		train, err := V3TaskFromExpr(fmt.Sprintf("hidden-train-%d", i), target, trains[i], holds[i], lib)
		if err != nil {
			return nil, err
		}
		holdTask, err := V3TaskFromExpr(fmt.Sprintf("hidden-hold-%d", i), target, holds[i], trains[i], lib)
		if err != nil {
			return nil, err
		}
		out = append(out, V3HiddenCase{ID: fmt.Sprintf("hidden-%d", i), Train: train, Holdout: holdTask})
	}
	return out, nil
}
