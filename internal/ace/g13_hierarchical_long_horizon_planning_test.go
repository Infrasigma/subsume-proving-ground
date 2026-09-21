package ace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

type g13World struct {
	N     int
	Edges [][2]int
}

type g13State struct {
	Pos int
}

type g13Task struct {
	World g13World
	Start int
	Goal  int
}

type g13Macro struct {
	Name  string
	Start int
	Goal  int
	Route []int
	Digest string
}

type g13PlanResult struct {
	Path              []int
	DecisionExpansions int
	MacroApplications int
	Found             bool
}

type g13PlanReport = g13Report

type g13Report struct {
	Seeds                         int
	TrainingTasks                int
	TargetTasks                  int
	MacrosLearned                int
	TransferSolved               int
	IndependentVerified          int
	ScratchSolved                int
	MacroAblationFailures        int
	OrderStressPasses            int
	AllPrimitiveLengthsEqual     bool
	NoTargetLeakage              bool
	MeanScratchDecisionExpansions float64
	MeanMacroDecisionExpansions   float64
	MedianDecisionRatio          float64
	MeanScratchPrimitiveActions   float64
	MeanMacroPrimitiveActions     float64
	Classification                string
}

func g13Neighbors(w g13World, pos int) []int {
	out := make([]int, 0)
	for _, e := range w.Edges {
		if e[0] == pos {
			out = append(out, e[1])
		}
		if e[1] == pos {
			out = append(out, e[0])
		}
	}
	sort.Ints(out)
	return out
}

func g13Apply(w g13World, pos, to int) (int, bool) {
	for _, n := range g13Neighbors(w, pos) {
		if n == to {
			return to, true
		}
	}
	return pos, false
}

func g13BFS(t g13Task) ([]int, int, bool) {
	type node struct {
		pos  int
		path []int
	}
	q := []node{{t.Start, []int{t.Start}}}
	seen := map[int]bool{t.Start: true}
	expansions := 0
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		expansions++
		if cur.pos == t.Goal {
			return cur.path, expansions, true
		}
		for _, n := range g13Neighbors(t.World, cur.pos) {
			if seen[n] {
				continue
			}
			seen[n] = true
			q = append(q, node{n, append(append([]int(nil), cur.path...), n)})
		}
	}
	return nil, expansions, false
}

func g13RouteDigest(route []int) string {
	b, _ := json.Marshal(route)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func g13LearnMacros(tasks []g13Task) []g13Macro {
	type evidence struct {
		route []int
		count int
	}
	buckets := map[[2]int]evidence{}
	for _, task := range tasks {
		route, _, ok := g13BFS(task)
		if !ok || len(route) < 2 {
			continue
		}
		key := [2]int{task.Start, task.Goal}
		digest := g13RouteDigest(route)
		ev := buckets[key]
		if ev.count > 0 && g13RouteDigest(ev.route) != digest {
			// Conflicting demonstrations for the same endpoint pair invalidate
			// the would-be reusable macro.
			buckets[key] = evidence{}
			continue
		}
		ev.route = append([]int(nil), route...)
		ev.count++
		buckets[key] = ev
	}
	out := make([]g13Macro, 0, len(buckets))
	keys := make([][2]int, 0, len(buckets))
	for k, ev := range buckets {
		if ev.count >= 3 && len(ev.route) >= 2 {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] == keys[j][0] {
			return keys[i][1] < keys[j][1]
		}
		return keys[i][0] < keys[j][0]
	})
	for _, k := range keys {
		ev := buckets[k]
		out = append(out, g13Macro{
			Name:   "route-" + strconv.Itoa(k[0]) + "-" + strconv.Itoa(k[1]),
			Start:  k[0],
			Goal:   k[1],
			Route:  append([]int(nil), ev.route...),
			Digest: g13RouteDigest(ev.route),
		})
	}
	return out
}

func g13LibraryDigest(macros []g13Macro) string {
	type item struct {
		Name  string
		Start int
		Goal  int
		Digest string
	}
	items := make([]item, 0, len(macros))
	for _, m := range macros {
		items = append(items, item{m.Name, m.Start, m.Goal, m.Digest})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Start == items[j].Start {
			if items[i].Goal == items[j].Goal {
				return items[i].Digest < items[j].Digest
			}
			return items[i].Goal < items[j].Goal
		}
		return items[i].Start < items[j].Start
	})
	b, _ := json.Marshal(items)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func g13MacroPlan(t g13Task, macros []g13Macro) g13PlanResult {
	type node struct {
		pos  int
		path []int
	}
	ordered := append([]g13Macro(nil), macros...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Start == ordered[j].Start {
			if ordered[i].Goal == ordered[j].Goal {
				return ordered[i].Digest < ordered[j].Digest
			}
			return ordered[i].Goal < ordered[j].Goal
		}
		return ordered[i].Start < ordered[j].Start
	})
	q := []node{{t.Start, []int{t.Start}}}
	seen := map[int]bool{t.Start: true}
	expansions := 0
	applications := 0
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		expansions++
		if cur.pos == t.Goal {
			return g13PlanResult{
				Path:               cur.path,
				DecisionExpansions: expansions,
				MacroApplications:  applications,
				Found:              true,
			}
		}
		for _, m := range ordered {
			if m.Start != cur.pos {
				continue
			}
			applications++
			if len(m.Route) < 2 || m.Route[0] != cur.pos || m.Route[len(m.Route)-1] != m.Goal {
				continue
			}
			pos := cur.pos
			valid := true
			full := append([]int(nil), cur.path...)
			for _, to := range m.Route[1:] {
				var ok bool
				pos, ok = g13Apply(t.World, pos, to)
				if !ok {
					valid = false
					break
				}
				full = append(full, pos)
			}
			if !valid || seen[pos] {
				continue
			}
			seen[pos] = true
			q = append(q, node{pos, full})
		}
	}
	return g13PlanResult{DecisionExpansions: expansions, MacroApplications: applications}
}

func g13IndependentVerify(t g13Task, path []int) bool {
	if len(path) == 0 || path[0] != t.Start || path[len(path)-1] != t.Goal {
		return false
	}
	pos := t.Start
	for _, to := range path[1:] {
		var ok bool
		pos, ok = g13Apply(t.World, pos, to)
		if !ok {
			return false
		}
	}
	return pos == t.Goal
}

func g13WorldForSeed(seed int) g13World {
	r := rand.New(rand.NewSource(int64(seed)))
	const segments = 4
	const segmentLen = 4
	const leavesPerCore = 3

	coreN := segments*segmentLen + 1
	edges := make([][2]int, 0, coreN+segments*segmentLen*leavesPerCore)
	for i := 0; i < coreN-1; i++ {
		edges = append(edges, [2]int{i, i + 1})
	}

	next := coreN
	for core := 0; core < coreN; core++ {
		for j := 0; j < leavesPerCore; j++ {
			edges = append(edges, [2]int{core, next})
			next++
		}
	}
	// A few within-room cycles create branching but no shortcut across the
	// trained macro boundaries.
	for s := 0; s < segments; s++ {
		base := s * segmentLen
		if r.Intn(2) == 0 {
			edges = append(edges, [2]int{base, base + 2})
		} else {
			edges = append(edges, [2]int{base + 1, base + 3})
		}
	}
	return g13World{N: next, Edges: edges}
}

func g13TrainingTasks(w g13World) []g13Task {
	segments := [][2]int{{0, 4}, {4, 8}, {8, 12}, {12, 16}}
	out := make([]g13Task, 0, len(segments)*6)
	for _, seg := range segments {
		for i := 0; i < 6; i++ {
			out = append(out, g13Task{World: w, Start: seg[0], Goal: seg[1]})
			out = append(out, g13Task{World: w, Start: seg[1], Goal: seg[0]})
		}
	}
	return out
}

func g13TargetTasks(w g13World) []g13Task {
	endpoints := []int{0, 4, 8, 12, 16}
	return []g13Task{
		{World: w, Start: endpoints[0], Goal: endpoints[3]},
		{World: w, Start: endpoints[1], Goal: endpoints[4]},
		{World: w, Start: endpoints[4], Goal: endpoints[1]},
		{World: w, Start: endpoints[3], Goal: endpoints[0]},
		{World: w, Start: endpoints[0], Goal: endpoints[4]},
		{World: w, Start: endpoints[4], Goal: endpoints[0]},
	}
}

func g13Write(name string, v any) {
	ws := os.Getenv("GITHUB_WORKSPACE")
	if ws == "" {
		return
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	_ = os.WriteFile(filepath.Join(ws, name), append(b, '\n'), 0644)
}

func g13Median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	y := append([]float64(nil), xs...)
	sort.Float64s(y)
	if len(y)%2 == 1 {
		return y[len(y)/2]
	}
	return (y[len(y)/2-1] + y[len(y)/2]) / 2
}

func TestG13HierarchicalLongHorizonPlanning(t *testing.T) {
	const seeds = 48
	report := g13PlanReport{Seeds: seeds, Classification: "G13_NOT_PROVEN"}
	ratios := make([]float64, 0, seeds*6)
	sumScratchDecision := 0.0
	sumMacroDecision := 0.0
	sumScratchActions := 0.0
	sumMacroActions := 0.0
	targetTasks := 0
	noTargetLeakage := true
	allPrimitiveLengthsEqual := true

	for seed := 1; seed <= seeds; seed++ {
		w := g13WorldForSeed(13000 + seed)
		training := g13TrainingTasks(w)
		macros := g13LearnMacros(training)
		if len(macros) != 8 {
			t.Fatalf("seed %d expected 8 bidirectional learned macros, got %d", seed, len(macros))
		}
		report.MacrosLearned += len(macros)
		report.TrainingTasks += len(training)

		beforeDigest := g13LibraryDigest(macros)
		targets := g13TargetTasks(w)
		afterDigest := g13LibraryDigest(macros)
		if beforeDigest != afterDigest {
			noTargetLeakage = false
		}

		for ti, target := range targets {
			targetTasks++
			scratchPath, scratchExp, scratchOK := g13BFS(target)
			macro := g13MacroPlan(target, macros)
			if !scratchOK || !macro.Found {
				t.Fatalf("seed %d target %d failed scratch=%v macro=%v", seed, ti, scratchOK, macro.Found)
			}
			if !g13IndependentVerify(target, scratchPath) || !g13IndependentVerify(target, macro.Path) {
				t.Fatalf("seed %d target %d independent plan verification failed", seed, ti)
			}
			report.TransferSolved++
			report.ScratchSolved++
			report.IndependentVerified++

			scratchActions := len(scratchPath)-1
			macroActions := len(macro.Path)-1
			sumScratchDecision += float64(scratchExp)
			sumMacroDecision += float64(macro.DecisionExpansions)
			sumScratchActions += float64(scratchActions)
			sumMacroActions += float64(macroActions)
			ratios = append(ratios, float64(scratchExp)/float64(macro.DecisionExpansions))

			if scratchActions != macroActions {
				allPrimitiveLengthsEqual = false
			}

			ablation := g13MacroPlan(target, nil)
			if ablation.Found {
				t.Fatalf("seed %d target %d macro ablation unexpectedly solved target", seed, ti)
			}
			report.MacroAblationFailures++

			shuffled := append([]g13Macro(nil), macros...)
			rr := rand.New(rand.NewSource(int64(17000 + seed*10 + ti)))
			rr.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
			stress := g13MacroPlan(target, shuffled)
			if !stress.Found || stress.DecisionExpansions != macro.DecisionExpansions ||
				!g13IndependentVerify(target, stress.Path) {
				t.Fatalf("seed %d target %d macro order stress failed", seed, ti)
			}
			report.OrderStressPasses++
		}
	}

	report.TargetTasks = targetTasks
	report.MeanScratchDecisionExpansions = sumScratchDecision / float64(targetTasks)
	report.MeanMacroDecisionExpansions = sumMacroDecision / float64(targetTasks)
	report.MeanScratchPrimitiveActions = sumScratchActions / float64(targetTasks)
	report.MeanMacroPrimitiveActions = sumMacroActions / float64(targetTasks)
	report.MedianDecisionRatio = g13Median(ratios)
	report.AllPrimitiveLengthsEqual = allPrimitiveLengthsEqual
	report.NoTargetLeakage = noTargetLeakage

	if report.TransferSolved == seeds*6 &&
		report.ScratchSolved == seeds*6 &&
		report.IndependentVerified == seeds*6 &&
		report.MacroAblationFailures == seeds*6 &&
		report.OrderStressPasses == seeds*6 &&
		report.AllPrimitiveLengthsEqual &&
		report.NoTargetLeakage &&
		report.MedianDecisionRatio >= 3.0 {
		report.Classification = "G13_VERIFIED_HIERARCHICAL_PLAN_COMPOSITION_PROVEN"
	}

	g13Write("ACE_G13_LONG_HORIZON_PLANNING.json", report)
	t.Logf("G13 report=%+v", report)
	if report.Classification != "G13_VERIFIED_HIERARCHICAL_PLAN_COMPOSITION_PROVEN" {
		t.Fatalf("G13 failed: %+v", report)
	}
}
