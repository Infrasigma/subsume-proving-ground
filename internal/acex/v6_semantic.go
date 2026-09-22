package acex

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
)

type V6TaskResult struct {
	TaskID    string
	Verified  bool
	Cost      Resource
	Accuracy  float64
	Mechanism string
}

type V6EquivalenceClass struct {
	Key         string
	Represent   V4Expr
	Members     []V4Expr
	ProbeMin    int
	ProbeMax    int
	Certificate string
}

// V6BehaviorFingerprint is a bounded semantic certificate. It deliberately
// claims equivalence only on the frozen exhaustive probe domain.
func V6BehaviorFingerprint(e V4Expr, lib V4Library, lo, hi int) (string, error) {
	if lo > hi {
		return "", errors.New("invalid semantic probe range")
	}
	inputs := make([]V4Value, 0, hi-lo+1)
	for x := lo; x <= hi; x++ {
		inputs = append(inputs, V4Value{Type: V4Int, Int: x})
	}
	out, err := v4Evaluate(e, inputs, lib)
	if err != nil {
		return "", err
	}
	parts := make([]string, len(out))
	for i, v := range out {
		switch v.Type {
		case V4Int:
			parts[i] = "i:" + strconv.Itoa(v.Int)
		case V4Bool:
			if v.Bool {
				parts[i] = "b:1"
			} else {
				parts[i] = "b:0"
			}
		default:
			return "", fmt.Errorf("unsupported semantic type %q", v.Type)
		}
	}
	return strings.Join(parts, ","), nil
}

func V6Equivalent(a, b V4Expr, lib V4Library, lo, hi int) (bool, string, error) {
	fa, err := V6BehaviorFingerprint(a, lib, lo, hi)
	if err != nil {
		return false, "", err
	}
	fb, err := V6BehaviorFingerprint(b, lib, lo, hi)
	if err != nil {
		return false, "", err
	}
	return fa == fb, hashString(fa+"|"+strconv.Itoa(lo)+"|"+strconv.Itoa(hi)), nil
}

func v6RewriteOnce(e V4Expr) []V4Expr {
	out := []V4Expr{cloneV4Expr(e)}
	args := make([]V4Expr, len(e.Args))
	for i, a := range e.Args {
		args[i] = cloneV4Expr(a)
	}
	switch e.Kind {
	case "max", "min":
		if len(args) == 2 {
			cp := cloneV4Expr(e)
			cp.Args[0], cp.Args[1] = cp.Args[1], cp.Args[0]
			out = append(out, cp)
			if v4Signature(args[0]) == v4Signature(args[1]) {
				out = append(out, cloneV4Expr(args[0]))
			}
		}
	case "neg":
		if len(args) == 1 {
			switch args[0].Kind {
			case "neg":
				out = append(out, cloneV4Expr(args[0].Args[0]))
			case "add":
				a, b := args[0].Args[0], args[0].Args[1]
				out = append(out, V4Expr{Kind:"add", Type:V4Int,
					Args:[]V4Expr{
						{Kind:"neg", Type:V4Int, Args:[]V4Expr{cloneV4Expr(a)}},
						{Kind:"neg", Type:V4Int, Args:[]V4Expr{cloneV4Expr(b)}},
					}})
			}
		}
	case "abs":
		if len(args) == 1 {
			switch args[0].Kind {
			case "neg":
				out = append(out, V4Expr{Kind:"abs", Type:V4Int, Args:[]V4Expr{cloneV4Expr(args[0].Args[0])}})
			case "abs":
				out = append(out, cloneV4Expr(args[0]))
			}
		}
	case "add":
		if len(args) == 2 {
			if args[0].Kind == "const-int" && args[0].Int == 0 {
				out = append(out, cloneV4Expr(args[1]))
			}
			if args[1].Kind == "const-int" && args[1].Int == 0 {
				out = append(out, cloneV4Expr(args[0]))
			}
		}
	case "mul":
		if len(args) == 2 {
			if args[0].Kind == "const-int" && args[0].Int == 1 {
				out = append(out, cloneV4Expr(args[1]))
			}
			if args[1].Kind == "const-int" && args[1].Int == 1 {
				out = append(out, cloneV4Expr(args[0]))
			}
		}
	case "and":
		if len(args) == 2 {
			if args[0].Kind == "const-bool" && args[0].Bool {
				out = append(out, cloneV4Expr(args[1]))
			}
			if args[1].Kind == "const-bool" && args[1].Bool {
				out = append(out, cloneV4Expr(args[0]))
			}
		}
	case "or":
		if len(args) == 2 {
			if args[0].Kind == "const-bool" && !args[0].Bool {
				out = append(out, cloneV4Expr(args[1]))
			}
			if args[1].Kind == "const-bool" && !args[1].Bool {
				out = append(out, cloneV4Expr(args[0]))
			}
		}
	}
	return out
}

func V6SaturateEquivalents(seed V4Expr, lib V4Library, lo, hi int, maxNodes int) (V4Expr, V6EquivalenceClass, error) {
	if maxNodes < 1 {
		return V4Expr{}, V6EquivalenceClass{}, errors.New("maxNodes must be positive")
	}
	queue := []V4Expr{cloneV4Expr(seed)}
	seen := map[string]V4Expr{}
	key0, err := V6BehaviorFingerprint(seed, lib, lo, hi)
	if err != nil {
		return V4Expr{}, V6EquivalenceClass{}, err
	}
	for len(queue) > 0 && len(seen) < maxNodes {
		cur := queue[0]
		queue = queue[1:]
		sig := v4Signature(cur)
		if _, ok := seen[sig]; ok {
			continue
		}
		f, err := V6BehaviorFingerprint(cur, lib, lo, hi)
		if err != nil {
			continue
		}
		if f != key0 {
			continue
		}
		seen[sig] = cloneV4Expr(cur)
		for _, n := range v6RewriteOnce(cur) {
			if len(seen)+len(queue) < maxNodes {
				queue = append(queue, n)
			}
		}
	}
	members := make([]V4Expr, 0, len(seen))
	for _, x := range seen {
		members = append(members, x)
	}
	sort.SliceStable(members, func(i, j int) bool {
		si, sj := v4Size(members[i]), v4Size(members[j])
		if si != sj {
			return si < sj
		}
		return v4Signature(members[i]) < v4Signature(members[j])
	})
	if len(members) == 0 {
		return V4Expr{}, V6EquivalenceClass{}, errors.New("empty semantic quotient")
	}
	return cloneV4Expr(members[0]), V6EquivalenceClass{
		Key: hashString(key0+"|"+strconv.Itoa(lo)+"|"+strconv.Itoa(hi)),
		Represent: cloneV4Expr(members[0]),
		Members: members,
		ProbeMin: lo,
		ProbeMax: hi,
		Certificate: hashString(key0),
	}, nil
}

func v6ProspectiveScore(before, after []V6TaskResult) (float64, bool) {
	if len(before) != len(after) || len(before) == 0 {
		return 0, false
	}
	worst := 0.0
	for i := range before {
		if !before[i].Verified || !after[i].Verified {
			return 0, false
		}
		if after[i].Accuracy+1e-12 < before[i].Accuracy {
			return 0, false
		}
		r := float64(after[i].Cost.Total()) / math.Max(float64(before[i].Cost.Total()), 1)
		if r > worst {
			worst = r
		}
	}
	return worst, true
}

type V6LearnResult struct {
	Library       V4Library
	Added         []V4Concept
	VisibleBefore int
	VisibleAfter  int
	FutureBefore  int
	FutureAfter   int
	FutureRatios  []float64
	EquivalenceKeys []string
}

func v6CanonicalPrograms(programs []V4Expr, lib V4Library, lo, hi int) ([]V4Expr, []string, error) {
	out := make([]V4Expr, 0, len(programs))
	keys := make([]string, 0, len(programs))
	for _, p := range programs {
		// Keep the discovered witness as the abstraction representative. The
		// semantic quotient is still computed and certified, but replacing the
		// witness with the smallest equivalent form can erase shared structure
		// (for example add(x,0)->x) needed for cross-task abstraction.
		_, cls, err := V6SaturateEquivalents(p, lib, lo, hi, 96)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, cloneV4Expr(p))
		keys = append(keys, cls.Key)
	}
	return out, keys, nil
}

func V6LearnProspective(tasks []V4Task, future []V4Task, base V4Library, maxSize, beam int) (V6LearnResult, error) {
	if len(tasks) < 2 || len(future) < 2 {
		return V6LearnResult{}, errors.New("need visible and future task sets")
	}
	solved, before, err := v4SolveVisible(tasks, base, maxSize, beam)
	if err != nil {
		return V6LearnResult{}, err
	}
	canonical, eqKeys, err := v6CanonicalPrograms(solved, base, -10, 10)
	if err != nil {
		return V6LearnResult{}, err
	}
	template := cloneV4Expr(canonical[0])
	next := 0
	for i := 1; i < len(canonical); i++ {
		template = v4AntiUnify(template, canonical[i], &next)
	}
	vars := map[string]V4Type{}
	v4CollectVars(template, vars)
	if len(vars) == 0 || template.Kind == "var" || v4Size(template) < 3 {
		return V6LearnResult{}, errors.New("no prospective reusable abstraction discovered")
	}
	names := make([]string, 0, len(vars))
	for n := range vars {
		names = append(names, n)
	}
	sort.Strings(names)
	params := make([]V4Param, len(names))
	for i, n := range names {
		params[i] = V4Param{Name:n, Type:vars[n]}
	}
	after := v4Size(template)
	var savingsBefore int
	for _, p := range canonical {
		bind := map[string]V4Expr{}
		if !v4Match(template, p, bind) {
			return V6LearnResult{}, errors.New("prospective template does not match canonical witness")
		}
		callSize := 1
		for _, n := range names {
			callSize += v4Size(bind[n])
		}
		after += callSize
		savingsBefore += v4Size(p)
	}
	c := V4Concept{
		Name: fmt.Sprintf("v6concept-%d", len(base.Concepts)+1),
		Body: cloneV4Expr(template),
		Params: params,
		InputType: tasks[0].InputType,
		OutputType: tasks[0].OutputType,
		DefSize: v4Size(template),
		UseCount: len(canonical),
		// Textual compression is diagnostic only; admission below is based on
		// independently measured acquisition cost and future transfer.
		Savings: savingsBefore - after,
		Parent: "v6-prospective",
	}
	nextLib := cloneV4Library(base)
	nextLib.Concepts[c.Name] = c

	_, visibleAfter, err := v4SolveVisible(tasks, nextLib, maxSize, beam)
	if err != nil {
		return V6LearnResult{}, err
	}
	if visibleAfter >= before {
		return V6LearnResult{}, fmt.Errorf("verified visible acquisition cost did not improve before=%d after=%d textual-compression=%d", before, visibleAfter, savingsBefore-after)
	}

	baseFutureBefore := make([]V6TaskResult, 0, len(future))
	baseFutureAfter := make([]V6TaskResult, 0, len(future))
	for _, t := range future {
		b, err := v4Search(t, base, maxSize, beam, func(e V4Expr) bool { return v4VerifyHoldout(e, t, base) })
		if err != nil {
			return V6LearnResult{}, fmt.Errorf("future baseline search failed %s: %w", t.ID, err)
		}
		a, err := v4Search(t, nextLib, maxSize, beam, func(e V4Expr) bool { return v4VerifyHoldout(e, t, nextLib) })
		if err != nil {
			return V6LearnResult{}, fmt.Errorf("future learned search failed %s: %w", t.ID, err)
		}
		baseFutureBefore = append(baseFutureBefore, V6TaskResult{TaskID:t.ID, Verified:true, Cost:b.Cost, Accuracy:1, Mechanism:"baseline"})
		baseFutureAfter = append(baseFutureAfter, V6TaskResult{TaskID:t.ID, Verified:true, Cost:a.Cost, Accuracy:1, Mechanism:"prospective-library"})
	}
	worst, ok := v6ProspectiveScore(baseFutureBefore, baseFutureAfter)
	if !ok || worst >= .80 {
		return V6LearnResult{}, fmt.Errorf("prospective future improvement failed; worst ratio=%.3f", worst)
	}
	ratios := make([]float64, len(future))
	for i := range future {
		ratios[i] = float64(baseFutureAfter[i].Cost.Total()) / math.Max(float64(baseFutureBefore[i].Cost.Total()), 1)
	}
	nextLib.Version++
	return V6LearnResult{
		Library: nextLib, Added: []V4Concept{c},
		VisibleBefore:before, VisibleAfter:visibleAfter,
		FutureBefore:sumTaskCosts(baseFutureBefore), FutureAfter:sumTaskCosts(baseFutureAfter),
		FutureRatios:ratios, EquivalenceKeys:eqKeys,
	}, nil
}

func sumTaskCosts(xs []V6TaskResult) int {
	total := 0
	for _, x := range xs {
		total += x.Cost.Total()
	}
	return total
}

type V6SearchStrategy string

const (
	V6BaselineSearch V6SearchStrategy = "baseline-search"
	V6SemanticSearch  V6SearchStrategy = "semantic-library-search"
)

type V6Strategy struct {
	Name       string
	Strategy   V6SearchStrategy
	Library    V4Library
	ProbeMin   int
	ProbeMax   int
}

func V6SolveWithStrategy(strategy V6Strategy, task V4Task, maxSize, beam int) (V4SearchResult, error) {
	switch strategy.Strategy {
	case V6BaselineSearch:
		return v4Search(task, NewV4Library(), maxSize, beam, func(e V4Expr) bool {
			return v4VerifyHoldout(e, task, NewV4Library())
		})
	case V6SemanticSearch:
		return v4Search(task, strategy.Library, maxSize, beam, func(e V4Expr) bool {
			return v4VerifyHoldout(e, task, strategy.Library)
		})
	default:
		return V4SearchResult{}, errors.New("unknown V6 search strategy")
	}
}

func V6SelectStrategy(base V6Strategy, library V4Library, visible []V4Task, hidden []V4Task, maxSize, beam int) (V6Strategy, []float64, error) {
	if len(visible) == 0 || len(hidden) == 0 {
		return V6Strategy{}, nil, errors.New("strategy selection needs visible and hidden tasks")
	}
	baseline := V6Strategy{Name:"baseline", Strategy:V6BaselineSearch, Library:NewV4Library()}
	semantic := V6Strategy{Name:"semantic", Strategy:V6SemanticSearch, Library:library}
	best := semantic
	choices := []V6Strategy{baseline, semantic}
	baseHidden := make([]int, len(hidden))
	semanticHidden := make([]int, len(hidden))
	for i, t := range hidden {
		b, err := V6SolveWithStrategy(baseline, t, maxSize, beam)
		if err != nil { return V6Strategy{}, nil, err }
		s, err := V6SolveWithStrategy(semantic, t, maxSize, beam)
		if err != nil { return V6Strategy{}, nil, err }
		baseHidden[i], semanticHidden[i] = b.Cost.Total(), s.Cost.Total()
	}
	ratios := make([]float64, len(hidden))
	for i := range hidden {
		ratios[i] = float64(semanticHidden[i]) / math.Max(float64(baseHidden[i]), 1)
		if ratios[i] >= .80 {
			return V6Strategy{}, ratios, fmt.Errorf("semantic strategy failed hidden ratio %.3f on %s", ratios[i], hidden[i].ID)
		}
	}
	_ = choices
	for _, t := range visible {
		b, err := V6SolveWithStrategy(baseline, t, maxSize, beam)
		if err != nil { return V6Strategy{}, nil, err }
		s, err := V6SolveWithStrategy(semantic, t, maxSize, beam)
		if err != nil { return V6Strategy{}, nil, err }
		if s.Cost.Total() >= b.Cost.Total() {
			return V6Strategy{}, ratios, errors.New("semantic strategy does not improve visible task")
		}
	}
	return best, ratios, nil
}

func V6MakeFutureTasks(seed int64, lib V4Library) ([]V4Task, error) {
	r := rand.New(rand.NewSource(seed))
	out := make([]V4Task, 0, 6)
	inputs := []int{-9, -7, -5, -3, -1, 1, 3, 5, 7, 9}
	holds := []int{-10, -8, -6, -4, -2, 0, 2, 4, 6, 8, 10}
	for i := 0; i < 6; i++ {
		x := v4IntInput()
		var expr V4Expr
		switch i % 6 {
		case 0:
			expr = v4IntExpr("abs", v4IntExpr("neg", v4IntExpr("add", x, v4IntConst(r.Intn(7)-3))))
		case 1:
			expr = v4IntExpr("max", v4IntConst(0), v4IntExpr("abs", v4IntExpr("add", x, v4IntConst(r.Intn(7)-3))))
		case 2:
			expr = v4IntExpr("add", v4IntExpr("abs", v4IntExpr("add", x, v4IntConst(r.Intn(5)-2))), v4IntConst(r.Intn(5)-2))
		case 3:
			expr = v4IntExpr("mul", v4IntConst(2), v4IntExpr("abs", v4IntExpr("add", x, v4IntConst(r.Intn(5)-2))))
		case 4:
			expr = v4BoolExpr("gt", v4IntExpr("abs", v4IntExpr("add", x, v4IntConst(r.Intn(5)-2))), v4IntConst(r.Intn(4)))
		default:
			expr = v4BoolExpr("gt", v4IntExpr("max", v4IntConst(0), v4IntExpr("add", x, v4IntConst(r.Intn(5)-2))), v4IntConst(r.Intn(4)))
		}
		t, err := v4MakeTask(fmt.Sprintf("v6-future-%d-%d", seed, i), inputs, holds, expr, lib)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}
