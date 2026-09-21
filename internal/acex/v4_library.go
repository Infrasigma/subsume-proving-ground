package acex

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type V4Type string

const (
	V4Int  V4Type = "Int"
	V4Bool V4Type = "Bool"
)

type V4Value struct {
	Type V4Type
	Int  int
	Bool bool
}

type V4Expr struct {
	Kind string
	Type V4Type
	Int int
	Bool bool
	Name string
	Args []V4Expr
}

type V4Task struct {
	ID string
	InputType V4Type
	OutputType V4Type
	TrainInputs []V4Value
	TrainOutput []V4Value
	HoldInputs []V4Value
	HoldOutput []V4Value
}

type V4Param struct {
	Name string
	Type V4Type
}

type V4Concept struct {
	Name string
	Body V4Expr
	Params []V4Param
	InputType V4Type
	OutputType V4Type
	DefSize int
	UseCount int
	Savings int
	Parent string
}

type V4Library struct {
	Version uint64
	Concepts map[string]V4Concept
}

type V4SearchResult struct {
	Program V4Expr
	Cost Resource
	Size int
	Found bool
}

type V4LearnResult struct {
	Library V4Library
	BeforeCost int
	AfterCost int
	Added []V4Concept
	SearchTrace []string
}

type V4HiddenCase struct {
	ID string
	Train V4Task
	Holdout V4Task
}

func NewV4Library() V4Library {
	return V4Library{Concepts: map[string]V4Concept{}}
}

func cloneV4Expr(e V4Expr) V4Expr {
	out := e
	out.Args = make([]V4Expr, len(e.Args))
	for i, a := range e.Args {
		out.Args[i] = cloneV4Expr(a)
	}
	return out
}

func cloneV4Library(in V4Library) V4Library {
	out := NewV4Library()
	out.Version = in.Version
	for k, v := range in.Concepts {
		vv := v
		vv.Body = cloneV4Expr(v.Body)
		vv.Params = append([]V4Param(nil), v.Params...)
		out.Concepts[k] = vv
	}
	return out
}

func v4Size(e V4Expr) int {
	n := 1
	for _, a := range e.Args {
		n += v4Size(a)
	}
	return n
}

func v4Signature(e V4Expr) string {
	switch e.Kind {
	case "input":
		return "input:" + string(e.Type)
	case "const-int":
		return "ci:" + strconv.Itoa(e.Int)
	case "const-bool":
		if e.Bool {
			return "cb:true"
		}
		return "cb:false"
	case "var":
		return "var:" + e.Name
	case "call":
		parts := make([]string, len(e.Args))
		for i, a := range e.Args {
			parts[i] = v4Signature(a)
		}
		return "call:" + e.Name + "(" + strings.Join(parts, ",") + ")"
	default:
		parts := make([]string, len(e.Args))
		for i, a := range e.Args {
			parts[i] = v4Signature(a)
		}
		return e.Kind + "(" + strings.Join(parts, ",") + ")"
	}
}

func v4ValueKey(v V4Value) string {
	switch v.Type {
	case V4Int:
		return "i:" + strconv.Itoa(v.Int)
	case V4Bool:
		if v.Bool {
			return "b:1"
		}
		return "b:0"
	default:
		return "?"
	}
}

func v4VectorKey(xs []V4Value) string {
	parts := make([]string, len(xs))
	for i, v := range xs {
		parts[i] = v4ValueKey(v)
	}
	return strings.Join(parts, ",")
}

func v4SameValue(a, b V4Value) bool {
	if a.Type != b.Type {
		return false
	}
	if a.Type == V4Int {
		return a.Int == b.Int
	}
	return a.Bool == b.Bool
}

func v4Eval(e V4Expr, in V4Value, lib V4Library, vars map[string]V4Value, stack map[string]bool) (V4Value, error) {
	switch e.Kind {
	case "input":
		if e.Type != in.Type {
			return V4Value{}, errors.New("input type mismatch")
		}
		return in, nil
	case "const-int":
		return V4Value{Type: V4Int, Int: e.Int}, nil
	case "const-bool":
		return V4Value{Type: V4Bool, Bool: e.Bool}, nil
	case "var":
		v, ok := vars[e.Name]
		if !ok || v.Type != e.Type {
			return V4Value{}, fmt.Errorf("unbound variable %q", e.Name)
		}
		return v, nil
	case "call":
		c, ok := lib.Concepts[e.Name]
		if !ok {
			return V4Value{}, fmt.Errorf("unknown concept %q", e.Name)
		}
		if stack[e.Name] {
			return V4Value{}, errors.New("recursive concept cycle")
		}
		if len(c.Params) != len(e.Args) {
			return V4Value{}, errors.New("concept arity mismatch")
		}
		local := map[string]V4Value{}
		for i, p := range c.Params {
			v, err := v4Eval(e.Args[i], in, lib, vars, stack)
			if err != nil || v.Type != p.Type {
				return V4Value{}, fmt.Errorf("concept argument %d type mismatch", i)
			}
			local[p.Name] = v
		}
		stack[e.Name] = true
		v, err := v4Eval(c.Body, in, lib, local, stack)
		delete(stack, e.Name)
		return v, err
	case "neg", "abs":
		if len(e.Args) != 1 || e.Args[0].Type != V4Int {
			return V4Value{}, errors.New("integer unary arity/type mismatch")
		}
		v, err := v4Eval(e.Args[0], in, lib, vars, stack)
		if err != nil {
			return V4Value{}, err
		}
		if e.Kind == "neg" {
			return V4Value{Type: V4Int, Int: -v.Int}, nil
		}
		return V4Value{Type: V4Int, Int: int(math.Abs(float64(v.Int)))}, nil
	case "not":
		if len(e.Args) != 1 || e.Args[0].Type != V4Bool {
			return V4Value{}, errors.New("not type")
		}
		v, err := v4Eval(e.Args[0], in, lib, vars, stack)
		if err != nil {
			return V4Value{}, err
		}
		return V4Value{Type: V4Bool, Bool: !v.Bool}, nil
	case "add", "mul", "max", "min":
		if len(e.Args) != 2 {
			return V4Value{}, errors.New("integer binary arity")
		}
		a, err := v4Eval(e.Args[0], in, lib, vars, stack)
		if err != nil {
			return V4Value{}, err
		}
		b, err := v4Eval(e.Args[1], in, lib, vars, stack)
		if err != nil || a.Type != V4Int || b.Type != V4Int {
			return V4Value{}, errors.New("integer binary type")
		}
		switch e.Kind {
		case "add":
			return V4Value{Type: V4Int, Int: a.Int + b.Int}, nil
		case "mul":
			return V4Value{Type: V4Int, Int: a.Int * b.Int}, nil
		case "max":
			if a.Int > b.Int {
				return a, nil
			}
			return b, nil
		default:
			if a.Int < b.Int {
				return a, nil
			}
			return b, nil
		}
	case "gt":
		if len(e.Args) != 2 {
			return V4Value{}, errors.New("gt arity")
		}
		a, err := v4Eval(e.Args[0], in, lib, vars, stack)
		if err != nil {
			return V4Value{}, err
		}
		b, err := v4Eval(e.Args[1], in, lib, vars, stack)
		if err != nil || a.Type != V4Int || b.Type != V4Int {
			return V4Value{}, errors.New("gt type")
		}
		return V4Value{Type: V4Bool, Bool: a.Int > b.Int}, nil
	case "and", "or":
		if len(e.Args) != 2 {
			return V4Value{}, errors.New("bool binary arity")
		}
		a, err := v4Eval(e.Args[0], in, lib, vars, stack)
		if err != nil {
			return V4Value{}, err
		}
		b, err := v4Eval(e.Args[1], in, lib, vars, stack)
		if err != nil || a.Type != V4Bool || b.Type != V4Bool {
			return V4Value{}, errors.New("bool binary type")
		}
		if e.Kind == "and" {
			return V4Value{Type: V4Bool, Bool: a.Bool && b.Bool}, nil
		}
		return V4Value{Type: V4Bool, Bool: a.Bool || b.Bool}, nil
	default:
		return V4Value{}, fmt.Errorf("unknown V4 op %q", e.Kind)
	}
}

func v4Evaluate(e V4Expr, inputs []V4Value, lib V4Library) ([]V4Value, error) {
	out := make([]V4Value, len(inputs))
	for i, x := range inputs {
		v, err := v4Eval(e, x, lib, map[string]V4Value{}, map[string]bool{})
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func v4MatchesTrain(e V4Expr, t V4Task, lib V4Library) bool {
	got, err := v4Evaluate(e, t.TrainInputs, lib)
	if err != nil || len(got) != len(t.TrainOutput) {
		return false
	}
	for i := range got {
		if !v4SameValue(got[i], t.TrainOutput[i]) {
			return false
		}
	}
	return true
}

func v4VerifyHoldout(e V4Expr, t V4Task, lib V4Library) bool {
	got, err := v4Evaluate(e, t.HoldInputs, lib)
	if err != nil || len(got) != len(t.HoldOutput) {
		return false
	}
	for i := range got {
		if !v4SameValue(got[i], t.HoldOutput[i]) {
			return false
		}
	}
	return true
}

func v4AtomicExprs(t V4Task) []V4Expr {
	out := []V4Expr{{Kind: "input", Type: t.InputType}}
	for i := -2; i <= 2; i++ {
		out = append(out, V4Expr{Kind: "const-int", Type: V4Int, Int: i})
	}
	out = append(out,
		V4Expr{Kind: "const-bool", Type: V4Bool, Bool: false},
		V4Expr{Kind: "const-bool", Type: V4Bool, Bool: true},
	)
	return out
}

func v4ArgAtoms(typ V4Type, inputType V4Type) []V4Expr {
	out := []V4Expr{}
	if inputType == typ {
		out = append(out, V4Expr{Kind: "input", Type: inputType})
	}
	if typ == V4Int {
		for i := -2; i <= 2; i++ {
			out = append(out, V4Expr{Kind: "const-int", Type: V4Int, Int: i})
		}
	} else {
		out = append(out,
			V4Expr{Kind: "const-bool", Type: V4Bool, Bool: false},
			V4Expr{Kind: "const-bool", Type: V4Bool, Bool: true},
		)
	}
	return out
}

func v4CallExprs(t V4Task, lib V4Library) []V4Expr {
	names := make([]string, 0, len(lib.Concepts))
	for name := range lib.Concepts {
		names = append(names, name)
	}
	sort.Strings(names)
	out := []V4Expr{}
	for _, name := range names {
		c := lib.Concepts[name]
		if c.OutputType != t.OutputType {
			continue
		}
		options := make([][]V4Expr, len(c.Params))
		sizeCount := 1
		possible := true
		for i, p := range c.Params {
			options[i] = v4ArgAtoms(p.Type, t.InputType)
			if len(options[i]) == 0 {
				possible = false
				break
			}
			sizeCount *= len(options[i])
			if sizeCount > 1500 {
				possible = false
				break
			}
		}
		if !possible || len(options) == 0 {
			continue
		}
		var walk func(int, []V4Expr)
		walk = func(idx int, args []V4Expr) {
			if len(out) >= 1500 {
				return
			}
			if idx == len(options) {
				cp := append([]V4Expr(nil), args...)
				out = append(out, V4Expr{Kind: "call", Type: c.OutputType, Name: name, Args: cp})
				return
			}
			for _, a := range options[idx] {
				walk(idx+1, append(args, a))
			}
		}
		walk(0, nil)
	}
	return out
}

func v4Search(t V4Task, lib V4Library, maxSize, beam int, accept func(V4Expr) bool) (V4SearchResult, error) {
	if maxSize < 1 || beam < 1 {
		return V4SearchResult{}, errors.New("invalid V4 search bounds")
	}
	bySize := map[int][]V4Expr{}
	seen := map[string]bool{}
	result := V4SearchResult{}
	add := func(e V4Expr) {
		size := v4Size(e)
		if size > maxSize {
			return
		}
		vals, err := v4Evaluate(e, t.TrainInputs, lib)
		if err != nil {
			return
		}
		key := string(e.Type) + "|" + strconv.Itoa(size) + "|" + v4VectorKey(vals)
		if seen[key] {
			return
		}
		seen[key] = true
		bySize[size] = append(bySize[size], e)
		result.Cost.Search++
	}
	for _, e := range v4AtomicExprs(t) {
		add(e)
	}
	for _, e := range v4CallExprs(t, lib) {
		add(e)
	}

	for size := 1; size <= maxSize; size++ {
		ints := []V4Expr{}
		bools := []V4Expr{}
		for _, e := range bySize[size] {
			if e.Type == V4Int {
				ints = append(ints, e)
			} else {
				bools = append(bools, e)
			}
		}
		sort.Slice(ints, func(i, j int) bool { return v4Signature(ints[i]) < v4Signature(ints[j]) })
		sort.Slice(bools, func(i, j int) bool { return v4Signature(bools[i]) < v4Signature(bools[j]) })
		if len(ints) > beam {
			ints = ints[:beam]
		}
		if len(bools) > beam {
			bools = bools[:beam]
		}
		check := func(e V4Expr) bool {
			if e.Type != t.OutputType || !v4MatchesTrain(e, t, lib) {
				return false
			}
			if accept != nil {
				result.Cost.Verify += len(t.TrainInputs) + len(t.HoldInputs)
				return accept(e)
			}
			return true
		}
		for _, e := range ints {
			if check(e) {
				result.Program = cloneV4Expr(e)
				result.Size = size
				result.Found = true
				return result, nil
			}
		}
		for _, e := range bools {
			if check(e) {
				result.Program = cloneV4Expr(e)
				result.Size = size
				result.Found = true
				return result, nil
			}
		}
		if size == maxSize {
			break
		}

		for _, e := range ints {
			add(V4Expr{Kind: "neg", Type: V4Int, Args: []V4Expr{e}})
			add(V4Expr{Kind: "abs", Type: V4Int, Args: []V4Expr{e}})
		}
		for _, e := range bools {
			add(V4Expr{Kind: "not", Type: V4Bool, Args: []V4Expr{e}})
		}
		for ls := 1; ls <= size; ls++ {
			rs := size - ls
			if rs < 1 {
				continue
			}
			var li, lb, ri, rb []V4Expr
			for _, e := range bySize[ls] {
				if e.Type == V4Int {
					li = append(li, e)
				} else {
					lb = append(lb, e)
				}
			}
			for _, e := range bySize[rs] {
				if e.Type == V4Int {
					ri = append(ri, e)
				} else {
					rb = append(rb, e)
				}
			}
			for _, a := range li {
				for _, b := range ri {
					add(V4Expr{Kind: "add", Type: V4Int, Args: []V4Expr{a, b}})
					add(V4Expr{Kind: "mul", Type: V4Int, Args: []V4Expr{a, b}})
					add(V4Expr{Kind: "max", Type: V4Int, Args: []V4Expr{a, b}})
					add(V4Expr{Kind: "min", Type: V4Int, Args: []V4Expr{a, b}})
					add(V4Expr{Kind: "gt", Type: V4Bool, Args: []V4Expr{a, b}})
				}
			}
			for _, a := range lb {
				for _, b := range rb {
					add(V4Expr{Kind: "and", Type: V4Bool, Args: []V4Expr{a, b}})
					add(V4Expr{Kind: "or", Type: V4Bool, Args: []V4Expr{a, b}})
				}
			}
		}
	}
	return result, errors.New("V4 search exhausted without verified program")
}

func v4AntiUnify(a, b V4Expr, next *int) V4Expr {
	if a.Type != b.Type || a.Kind != b.Kind || a.Kind == "var" || len(a.Args) != len(b.Args) {
		name := fmt.Sprintf("p%d", *next)
		*next = *next + 1
		return V4Expr{Kind: "var", Type: a.Type, Name: name}
	}
	switch a.Kind {
	case "const-int":
		if a.Int != b.Int {
			name := fmt.Sprintf("p%d", *next)
			*next = *next + 1
			return V4Expr{Kind: "var", Type: a.Type, Name: name}
		}
	case "const-bool":
		if a.Bool != b.Bool {
			name := fmt.Sprintf("p%d", *next)
			*next = *next + 1
			return V4Expr{Kind: "var", Type: a.Type, Name: name}
		}
	case "call":
		if a.Name != b.Name {
			name := fmt.Sprintf("p%d", *next)
			*next = *next + 1
			return V4Expr{Kind: "var", Type: a.Type, Name: name}
		}
	}
	out := cloneV4Expr(a)
	for i := range a.Args {
		out.Args[i] = v4AntiUnify(a.Args[i], b.Args[i], next)
	}
	return out
}

func v4CollectVars(e V4Expr, out map[string]V4Type) {
	if e.Kind == "var" {
		out[e.Name] = e.Type
		return
	}
	for _, a := range e.Args {
		v4CollectVars(a, out)
	}
}

func v4Match(template, concrete V4Expr, bindings map[string]V4Expr) bool {
	if template.Kind == "var" {
		if concrete.Type != template.Type {
			return false
		}
		if old, ok := bindings[template.Name]; ok {
			return v4Signature(old) == v4Signature(concrete)
		}
		bindings[template.Name] = cloneV4Expr(concrete)
		return true
	}
	if template.Kind != concrete.Kind || template.Type != concrete.Type || len(template.Args) != len(concrete.Args) {
		return false
	}
	switch template.Kind {
	case "const-int":
		return template.Int == concrete.Int
	case "const-bool":
		return template.Bool == concrete.Bool
	case "call":
		if template.Name != concrete.Name {
			return false
		}
	}
	for i := range template.Args {
		if !v4Match(template.Args[i], concrete.Args[i], bindings) {
			return false
		}
	}
	return true
}

func v4SolveVisible(tasks []V4Task, lib V4Library, maxSize, beam int) ([]V4Expr, int, error) {
	solved := make([]V4Expr, 0, len(tasks))
	cost := 0
	for _, t := range tasks {
		res, err := v4Search(t, lib, maxSize, beam, func(e V4Expr) bool {
			return v4VerifyHoldout(e, t, lib)
		})
		if err != nil {
			return nil, cost, err
		}
		solved = append(solved, res.Program)
		cost += res.Cost.Total()
	}
	return solved, cost, nil
}

func v4DiscoverConcept(tasks []V4Task, solved []V4Expr, base V4Library) (V4Concept, error) {
	if len(tasks) < 2 || len(solved) != len(tasks) {
		return V4Concept{}, errors.New("need multiple solved tasks")
	}
	next := 0
	template := cloneV4Expr(solved[0])
	for i := 1; i < len(solved); i++ {
		template = v4AntiUnify(template, solved[i], &next)
	}
	vars := map[string]V4Type{}
	v4CollectVars(template, vars)
	if len(vars) == 0 || template.Kind == "var" || v4Size(template) < 3 {
		return V4Concept{}, errors.New("anti-unification produced no nontrivial parameterized concept")
	}
	names := make([]string, 0, len(vars))
	for n := range vars {
		names = append(names, n)
	}
	sort.Strings(names)
	params := make([]V4Param, len(names))
	for i, n := range names {
		params[i] = V4Param{Name: n, Type: vars[n]}
	}
	before := 0
	after := v4Size(template)
	for _, p := range solved {
		bind := map[string]V4Expr{}
		if !v4Match(template, p, bind) {
			return V4Concept{}, errors.New("anti-unified template does not match solved witness")
		}
		callSize := 1
		for _, n := range names {
			callSize += v4Size(bind[n])
		}
		after += callSize
		before += v4Size(p)
	}
	savings := before - after
	if savings <= 0 {
		return V4Concept{}, fmt.Errorf("concept compression nonpositive before=%d after=%d", before, after)
	}
	return V4Concept{
		Name: fmt.Sprintf("v4concept-%d", len(base.Concepts)+1),
		Body: template,
		Params: params,
		InputType: tasks[0].InputType,
		OutputType: tasks[0].OutputType,
		DefSize: v4Size(template),
		UseCount: len(solved),
		Savings: savings,
		Parent: "base",
	}, nil
}

func V4LearnConcept(tasks []V4Task, base V4Library, maxSize, beam int) (V4LearnResult, error) {
	solved, before, err := v4SolveVisible(tasks, base, maxSize, beam)
	if err != nil {
		return V4LearnResult{Library: base, BeforeCost: before, AfterCost: before}, err
	}
	c, err := v4DiscoverConcept(tasks, solved, base)
	if err != nil {
		return V4LearnResult{Library: base, BeforeCost: before, AfterCost: before}, err
	}
	next := cloneV4Library(base)
	next.Concepts[c.Name] = c
	_, after, err := v4SolveVisible(tasks, next, maxSize, beam)
	if err != nil {
		return V4LearnResult{Library: base, BeforeCost: before, AfterCost: after}, err
	}
	if after >= before {
		return V4LearnResult{Library: base, BeforeCost: before, AfterCost: after},
			fmt.Errorf("concept did not reduce verified acquisition cost before=%d after=%d", before, after)
	}
	next.Version++
	return V4LearnResult{
		Library: next,
		BeforeCost: before,
		AfterCost: after,
		Added: []V4Concept{c},
		SearchTrace: []string{c.Name + "=" + v4Signature(c.Body)},
	}, nil
}

func V4VerifyHiddenSuccessors(lib, base V4Library, hidden []V4HiddenCase, maxSize, beam int) ([]float64, int, int, error) {
	if len(hidden) < 6 {
		return nil, 0, 0, errors.New("need at least six hidden cases")
	}
	ratios := make([]float64, 0, len(hidden))
	before, after := 0, 0
	for _, hc := range hidden {
		baseSearch, err := v4Search(hc.Train, base, maxSize, beam, func(e V4Expr) bool {
			return v4VerifyHoldout(e, hc.Train, base) && v4VerifyHoldout(e, hc.Holdout, base)
		})
		if err != nil {
			return ratios, before, after, fmt.Errorf("baseline hidden search failed %s: %w", hc.ID, err)
		}
		learnedSearch, err := v4Search(hc.Train, lib, maxSize, beam, func(e V4Expr) bool {
			return v4VerifyHoldout(e, hc.Train, lib) && v4VerifyHoldout(e, hc.Holdout, lib)
		})
		if err != nil {
			return ratios, before, after, fmt.Errorf("learned hidden search failed %s: %w", hc.ID, err)
		}
		before += baseSearch.Cost.Total()
		after += learnedSearch.Cost.Total()
		r := float64(learnedSearch.Cost.Total()) / math.Max(float64(baseSearch.Cost.Total()), 1)
		ratios = append(ratios, r)
		if r >= 0.75 {
			return ratios, before, after, fmt.Errorf("hidden ratio %.3f >= 0.75 for %s", r, hc.ID)
		}
	}
	return ratios, before, after, nil
}

func v4MakeTask(id string, in, hold []int, expr V4Expr, lib V4Library) (V4Task, error) {
	xs := make([]V4Value, len(in))
	hs := make([]V4Value, len(hold))
	for i, x := range in {
		xs[i] = V4Value{Type: V4Int, Int: x}
	}
	for i, x := range hold {
		hs[i] = V4Value{Type: V4Int, Int: x}
	}
	y, err := v4Evaluate(expr, xs, lib)
	if err != nil {
		return V4Task{}, err
	}
	h, err := v4Evaluate(expr, hs, lib)
	if err != nil {
		return V4Task{}, err
	}
	if len(y) == 0 {
		return V4Task{}, errors.New("empty task output")
	}
	return V4Task{ID: id, InputType: V4Int, OutputType: y[0].Type, TrainInputs: xs, TrainOutput: y, HoldInputs: hs, HoldOutput: h}, nil
}

func v4IntExpr(k string, args ...V4Expr) V4Expr {
	return V4Expr{Kind: k, Type: V4Int, Args: append([]V4Expr(nil), args...)}
}

func v4BoolExpr(k string, args ...V4Expr) V4Expr {
	return V4Expr{Kind: k, Type: V4Bool, Args: append([]V4Expr(nil), args...)}
}

func v4IntInput() V4Expr {
	return V4Expr{Kind: "input", Type: V4Int}
}

func v4IntConst(v int) V4Expr {
	return V4Expr{Kind: "const-int", Type: V4Int, Int: v}
}

func v4ConceptCall(c V4Concept, vals map[string]V4Expr) V4Expr {
	args := make([]V4Expr, len(c.Params))
	for i, p := range c.Params {
		args[i] = cloneV4Expr(vals[p.Name])
	}
	return V4Expr{Kind: "call", Type: c.OutputType, Name: c.Name, Args: args}
}

func V4MakeFirstIntTasks(seed int64) ([]V4Task, error) {
	delta := int(seed%3) - 1
	specs := [][3]int{{-1, 2, 1}, {0, 2, -1}, {-2, 1, delta}}
	defs := make([]V4Expr, 0, 3)
	x := v4IntInput()
	for _, s := range specs {
		clamped := v4IntExpr("add", x, v4IntConst(s[2]))
		body := v4IntExpr("max", v4IntConst(s[0]), v4IntExpr("min", v4IntConst(s[1]), clamped))
		defs = append(defs, body)
	}
	inputs := []int{-6, -4, -2, -1, 0, 2, 4}
	holds := []int{-7, -5, -3, 1, 3, 5, 7}
	out := make([]V4Task, 0, 3)
	for i, d := range defs {
		t, err := v4MakeTask(fmt.Sprintf("v4-int-g1-%d", i), inputs, holds, d, NewV4Library())
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func V4MakeFirstBoolTasks(seed int64) ([]V4Task, error) {
	offset := int(seed%3) - 1
	specs := [][2]int{{offset, 0}, {1, 1}, {-1, -1}}
	defs := make([]V4Expr, 0, 3)
	x := v4IntInput()
	for _, s := range specs {
		defs = append(defs, v4BoolExpr("gt", v4IntExpr("add", x, v4IntConst(s[0])), v4IntConst(s[1])))
	}
	inputs := []int{-5, -3, -1, 0, 1, 3, 5}
	holds := []int{-6, -4, -2, 2, 4, 6}
	out := make([]V4Task, 0, 3)
	for i, d := range defs {
		t, err := v4MakeTask(fmt.Sprintf("v4-bool-g1-%d", i), inputs, holds, d, NewV4Library())
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func V4MakeSecondIntTasks(first V4Concept) ([]V4Task, error) {
	if len(first.Params) != 3 {
		return nil, errors.New("unexpected first concept arity")
	}
	vals := []map[string]V4Expr{
		{"p0": v4IntConst(-1), "p1": v4IntConst(2), "p2": v4IntConst(1)},
		{"p0": v4IntConst(0), "p1": v4IntConst(2), "p2": v4IntConst(-1)},
		{"p0": v4IntConst(-2), "p1": v4IntConst(1), "p2": v4IntConst(0)},
	}
	defs := make([]V4Expr, 0, 3)
	for i, m := range vals {
		c := v4ConceptCall(first, m)
		defs = append(defs, v4IntExpr("add", c, v4IntConst([]int{1, -1, 2}[i])))
	}
	inputs := []int{-7, -5, -3, -1, 0, 2, 4, 6}
	holds := []int{-8, -6, -4, -2, 1, 3, 5, 7}
	out := make([]V4Task, 0, 3)
	lib := NewV4Library()
	lib.Concepts[first.Name] = first
	for i, d := range defs {
		t, err := v4MakeTask(fmt.Sprintf("v4-int-g2-%d", i), inputs, holds, d, lib)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func V4MakeHiddenSuccessors(seed int64, intSecond V4Concept, boolFirst V4Concept, lib V4Library) ([]V4HiddenCase, error) {
	s := int(seed)
	intTargets := []V4Expr{
		v4ConceptCall(intSecond, map[string]V4Expr{"p0": v4IntConst((s%3)-1), "p1": v4IntConst(2), "p2": v4IntConst(1), "p3": v4IntConst(1)}),
		v4ConceptCall(intSecond, map[string]V4Expr{"p0": v4IntConst(0), "p1": v4IntConst((s%3)+1), "p2": v4IntConst(-1), "p3": v4IntConst(-1)}),
		v4ConceptCall(intSecond, map[string]V4Expr{"p0": v4IntConst(-2), "p1": v4IntConst(1), "p2": v4IntConst(0), "p3": v4IntConst(2)}),
	}
	boolTargets := []V4Expr{
		v4ConceptCall(boolFirst, map[string]V4Expr{"p0": v4IntConst((s%3)-1), "p1": v4IntConst(0)}),
		v4ConceptCall(boolFirst, map[string]V4Expr{"p0": v4IntConst(-1), "p1": v4IntConst(1)}),
		v4ConceptCall(boolFirst, map[string]V4Expr{"p0": v4IntConst(1), "p1": v4IntConst(-1)}),
	}
	all := make([]V4HiddenCase, 0, 6)
	in1 := []int{-9, -6, -3, 0, 3, 6, 9}
	hold1 := []int{-10, -7, -4, -1, 2, 5, 8, 11}
	libAll := cloneV4Library(lib)
	for i, d := range intTargets {
		t, err := v4MakeTask(fmt.Sprintf("v4-hidden-int-%d", i), in1, hold1, d, libAll)
		if err != nil {
			return nil, err
		}
		alt, err := v4MakeTask(fmt.Sprintf("v4-hidden-int-hold-%d", i), hold1, in1, d, libAll)
		if err != nil {
			return nil, err
		}
		all = append(all, V4HiddenCase{ID: fmt.Sprintf("v4-hidden-int-%d", i), Train: t, Holdout: alt})
	}
	in2 := []int{-8, -5, -2, 1, 4, 7, 10}
	hold2 := []int{-9, -6, -3, 0, 3, 6, 9, 12}
	for i, d := range boolTargets {
		t, err := v4MakeTask(fmt.Sprintf("v4-hidden-bool-%d", i), in2, hold2, d, libAll)
		if err != nil {
			return nil, err
		}
		alt, err := v4MakeTask(fmt.Sprintf("v4-hidden-bool-hold-%d", i), hold2, in2, d, libAll)
		if err != nil {
			return nil, err
		}
		all = append(all, V4HiddenCase{ID: fmt.Sprintf("v4-hidden-bool-%d", i), Train: t, Holdout: alt})
	}
	return all, nil
}
