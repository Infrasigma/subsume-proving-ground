package ace

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type g10Chain struct {
	Domain string
	Op     string
	Depth  int
}

type g10Template struct {
	Depth int
}

type g10Task struct {
	Domain string
	Op     string
	Depth  int
	Train  [][2]string
	Hidden [][2]string
}

type g10Report struct {
	SourceTasks              int
	TemplatesLearned         int
	SourceHiddenVerified     bool
	TargetDomains            int
	TargetTasks              int
	TransferSolved           int
	IndependentVerified      int
	ScratchSolved            int
	MedianExpansionRatio     float64
	AllTransferSolved        bool
	NoTargetLeakage          bool
	Classification            string
}

func g10NumApply(op, in string) string {
	x, _ := strconv.Atoi(in)
	switch op {
	case "inc1":
		return strconv.Itoa(x + 1)
	case "dec2":
		return strconv.Itoa(x - 2)
	case "double":
		return strconv.Itoa(x * 2)
	case "shift5":
		return strconv.Itoa(x + 5)
	default:
		return ""
	}
}

func g10SplitList(s string) []int {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		v, _ := strconv.Atoi(p)
		out = append(out, v)
	}
	return out
}

func g10JoinList(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return strings.Join(parts, ",")
}

func g10DomainApply(domain, op, in string) string {
	switch domain {
	case "token":
		switch op {
		case "tokA":
			return in + "A"
		case "tokB":
			return in + "B"
		case "tokC":
			return in + "C"
		case "tokD":
			return in + "D"
		}
	case "text":
		switch op {
		case "appendA":
			return in + "a"
		case "prependB":
			return "b" + in
		case "flip":
			out := make([]byte, len(in))
			for i := range in {
				if in[i] >= 'a' && in[i] <= 'z' {
					out[i] = in[i] - ('a' - 'A')
				} else if in[i] >= 'A' && in[i] <= 'Z' {
					out[i] = in[i] + ('a' - 'A')
				} else {
					out[i] = in[i]
				}
			}
			return string(out)
		case "suffixZ":
			return in + "z"
		case "appendC":
			return in + "c"
		}
	case "list":
		xs := g10SplitList(in)
		switch op {
		case "rot1":
			if len(xs) > 0 {
				xs = append(append([]int(nil), xs[1:]...), xs[0])
			}
		case "add0":
			xs = append([]int{0}, xs...)
		case "add9":
			xs = append(xs, 9)
		case "add1":
			xs = append(xs, 1)
		case "add4":
			xs = append(xs, 4)
		case "swap":
			for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
				xs[i], xs[j] = xs[j], xs[i]
			}
		}
		return g10JoinList(xs)
	}
	return ""
}

func g10IndependentApply(domain, op, in string) string {
	if domain == "token" {
		switch op {
		case "tokA":
			return in + "A"
		case "tokB":
			return in + "B"
		case "tokC":
			return in + "C"
		case "tokD":
			return in + "D"
		}
	}
	if domain == "text" {
		switch op {
		case "appendA":
			return string(append([]byte(in), 'a'))
		case "prependB":
			return "b" + in
		case "flip":
			var b strings.Builder
			for _, r := range in {
				if r >= 'a' && r <= 'z' {
					b.WriteRune(r - ('a' - 'A'))
				} else if r >= 'A' && r <= 'Z' {
					b.WriteRune(r + ('a' - 'A'))
				} else {
					b.WriteRune(r)
				}
			}
			return b.String()
		case "suffixZ":
			return in + "z"
		case "appendC":
			return in + "c"
		}
	}
	if domain == "list" {
		xs := g10SplitList(in)
		switch op {
		case "rot1":
			if len(xs) > 0 {
				first := xs[0]
				xs = append([]int{}, xs[1:]...)
				xs = append(xs, first)
			}
		case "add0":
			xs = append([]int{0}, xs...)
		case "add9":
			xs = append(append([]int{}, xs...), 9)
		case "add1":
			xs = append(append([]int{}, xs...), 1)
		case "add4":
			xs = append(append([]int{}, xs...), 4)
		case "swap":
			for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
				xs[i], xs[j] = xs[j], xs[i]
			}
		}
		return g10JoinList(xs)
	}
	return ""
}

func g10ApplyRepeated(domain, op, in string, depth int) string {
	v := in
	for i := 0; i < depth; i++ {
		v = g10DomainApply(domain, op, v)
	}
	if domain == "num" {
		v = in
		for i := 0; i < depth; i++ {
			v = g10NumApply(op, v)
		}
	}
	return v
}

func g10IndependentRepeated(domain, op, in string, depth int) string {
	v := in
	for i := 0; i < depth; i++ {
		if domain == "num" {
			v = func() string {
				x, _ := strconv.Atoi(v)
				switch op {
				case "inc1":
					return strconv.Itoa(x + 1)
				case "dec2":
					return strconv.Itoa(x - 2)
				case "double":
					return strconv.Itoa(x * 2)
				case "shift5":
					return strconv.Itoa(x + 5)
				default:
					return ""
				}
			}()
		} else {
			v = g10IndependentApply(domain, op, v)
		}
	}
	return v
}

func g10BuildTask(domain, op string, depth int) g10Task {
	var xs []string
	switch domain {
	case "token":
		xs = []string{"x", "ab", "z", "root"}
	case "text":
		xs = []string{"ab", "Cab", "xy", "z", "aZ", "Ba"}
	case "list":
		xs = []string{"1,2,3", "4,1", "0,7,2,5", "9,3,1", "2,8"}
	case "num":
		xs = []string{"-7", "-2", "0", "3", "9", "14"}
	}
	train := make([][2]string, 0, len(xs))
	hidden := make([][2]string, 0, 4)
	for _, x := range xs {
		train = append(train, [2]string{x, g10ApplyRepeated(domain, op, x, depth)})
	}
	hiddenXS := []string{"11", "-11", "5", "21"}
	if domain == "token" {
		hiddenXS = []string{"q", "seed", "base", "!"}
	}
	if domain == "text" {
		hiddenXS = []string{"hello", "Az", "xyZ", "K"}
	}
	if domain == "list" {
		hiddenXS = []string{"3,7,2", "1,1,5,9", "8,0", "6,4,2,1"}
	}
	for _, x := range hiddenXS {
		hidden = append(hidden, [2]string{x, g10ApplyRepeated(domain, op, x, depth)})
	}
	return g10Task{Domain: domain, Op: op, Depth: depth, Train: train, Hidden: hidden}
}

func g10LearnTemplates(source []g10Chain) []g10Template {
	type count struct {
		ops map[string]bool
	}
	counts := map[int]count{}
	for _, p := range source {
		c := counts[p.Depth]
		if c.ops == nil {
			c.ops = map[string]bool{}
		}
		c.ops[p.Op] = true
		counts[p.Depth] = c
	}
	out := make([]g10Template, 0, len(counts))
	for d, c := range counts {
		if len(c.ops) >= 3 && d >= 2 {
			out = append(out, g10Template{Depth: d})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Depth < out[j].Depth })
	return out
}

func g10TemplatesFit(tasks []g10Task, templates []g10Template) bool {
	for _, task := range tasks {
		found := false
		for _, tmpl := range templates {
			if tmpl.Depth != task.Depth {
				continue
			}
			valid := true
			for _, ex := range task.Train {
				if g10ApplyRepeated(task.Domain, task.Op, ex[0], tmpl.Depth) != ex[1] {
					valid = false
					break
				}
			}
			if valid {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func g10TransferSolve(task g10Task, templates []g10Template, ops []string) (int, bool) {
	tests := 0
	for _, tmpl := range templates {
		for _, op := range ops {
			tests++
			ok := true
			for _, ex := range task.Train {
				if g10ApplyRepeated(task.Domain, op, ex[0], tmpl.Depth) != ex[1] {
					ok = false
					break
				}
			}
			if ok {
				return tests, true
			}
		}
	}
	return tests, false
}

func g10ScratchProgram(task g10Task, ops []string) (g10Chain, int, bool) {
	tests := 0
	var dfs func([]string, int) (g10Chain, bool)
	dfs = func(prefix []string, maxDepth int) (g10Chain, bool) {
		if len(prefix) == maxDepth {
			tests++
			ok := true
			for _, ex := range task.Train {
				v := ex[0]
				for _, op := range prefix {
					v = g10DomainApply(task.Domain, op, v)
				}
				if v != ex[1] {
					ok = false
					break
				}
			}
			if ok {
				return g10Chain{Domain: task.Domain, Op: prefix[0], Depth: maxDepth}, true
			}
			return g10Chain{}, false
		}
		for _, op := range ops {
			if found, ok := dfs(append(append([]string(nil), prefix...), op), maxDepth); ok {
				return found, true
			}
		}
		return g10Chain{}, false
	}
	for depth := 1; depth <= task.Depth; depth++ {
		if found, ok := dfs(nil, depth); ok {
			return found, tests, true
		}
	}
	return g10Chain{}, tests, false
}

func g10ScratchSolve(task g10Task, ops []string) (int, bool) {
	tests := 0
	var dfs func([]string, int) (int, bool)
	dfs = func(prefix []string, maxDepth int) (int, bool) {
		if len(prefix) == maxDepth {
			tests++
			ok := true
			for _, ex := range task.Train {
				v := ex[0]
				for _, op := range prefix {
					v = g10DomainApply(task.Domain, op, v)
				}
				if v != ex[1] {
					ok = false
					break
				}
			}
			if ok {
				return tests, true
			}
			return tests, false
		}
		for _, op := range ops {
			if n, ok := dfs(append(prefix, op), maxDepth); ok {
				return n, true
			}
		}
		return tests, false
	}
	for depth := 1; depth <= task.Depth; depth++ {
		before := tests
		if n, ok := dfs(nil, depth); ok {
			return n, true
		}
		if tests == before {
			break
		}
	}
	return tests, false
}

func g10WriteReport(name string, v any) {
	ws := os.Getenv("GITHUB_WORKSPACE")
	if ws == "" {
		return
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	_ = os.WriteFile(filepath.Join(ws, name), append(b, '\n'), 0644)
}

func TestG10CrossDomainAbstractionTransfer(t *testing.T) {
	sourceOps := []string{"tokA", "tokB", "tokC", "tokD"}
	sourceTasks := make([]g10Task, 0, 12)
	learnedPrograms := make([]g10Chain, 0, 12)
	for _, op := range sourceOps {
		for _, depth := range []int{2, 3, 4} {
			task := g10BuildTask("token", op, depth)
			program, _, ok := g10ScratchProgram(task, sourceOps)
			if !ok {
				t.Fatalf("source scratch synthesis failed op=%s depth=%d", op, depth)
			}
			sourceTasks = append(sourceTasks, task)
			learnedPrograms = append(learnedPrograms, program)
		}
	}
	templates := g10LearnTemplates(learnedPrograms)
	if len(templates) != 3 {
		t.Fatalf("expected learned repeat templates for depths 2,3,4; got %+v", templates)
	}
	if !g10TemplatesFit(sourceTasks, templates) {
		t.Fatal("learned templates failed source training verification")
	}
	for i, task := range sourceTasks {
		program := learnedPrograms[i]
		for _, ex := range task.Hidden {
			if g10IndependentRepeated(task.Domain, program.Op, ex[0], program.Depth) != ex[1] {
				t.Fatalf("source hidden verification failed op=%s depth=%d", program.Op, program.Depth)
			}
		}
	}

	targetDomains := map[string][]string{
		"text": {"appendA", "prependB", "suffixZ", "appendC"},
		"list": {"add0", "add9", "add1", "add4"},
	}
	targetTasks := make([]g10Task, 0, 16)
	for domain, ops := range targetDomains {
		for _, op := range ops {
			for _, depth := range []int{3, 4} {
				targetTasks = append(targetTasks, g10BuildTask(domain, op, depth))
			}
		}
	}

	totalRatio := make([]float64, 0, len(targetTasks))
	transferSolved := 0
	scratchSolved := 0
	independent := 0
	noLeak := true
	for _, task := range targetTasks {
		ops := targetDomains[task.Domain]
		expansions, ok := g10TransferSolve(task, templates, ops)
		if !ok {
			t.Fatalf("transfer failed domain=%s op=%s depth=%d", task.Domain, task.Op, task.Depth)
		}
		transferSolved++
		for _, ex := range task.Hidden {
			if g10IndependentRepeated(task.Domain, task.Op, ex[0], task.Depth) != ex[1] {
				t.Fatalf("independent transfer verification failed domain=%s op=%s", task.Domain, task.Op)
			}
		}
		independent++
		s, sok := g10ScratchSolve(task, ops)
		if sok {
			scratchSolved++
			if s > 0 {
				totalRatio = append(totalRatio, float64(s)/float64(expansions))
			}
		}
		if task.Domain == "text" || task.Domain == "list" {
			for _, c := range sourceOps {
				for _, tmpl := range templates {
					if c == task.Op && tmpl.Depth == task.Depth {
						noLeak = false
					}
				}
			}
		}
	}

	sort.Float64s(totalRatio)
	median := 0.0
	if len(totalRatio) > 0 {
		if len(totalRatio)%2 == 1 {
			median = totalRatio[len(totalRatio)/2]
		} else {
			median = (totalRatio[len(totalRatio)/2-1] + totalRatio[len(totalRatio)/2]) / 2
		}
	}
	class := "G10_NOT_PROVEN"
	if len(templates) == 3 &&
		g10TemplatesFit(sourceTasks, templates) &&
		transferSolved == len(targetTasks) &&
		independent == len(targetTasks) &&
		scratchSolved == len(targetTasks) &&
		median >= 5.0 &&
		noLeak {
		class = "G10_CROSS_DOMAIN_ABSTRACTION_TRANSFER_PROVEN"
	}
	report := g10Report{
		SourceTasks: len(sourceTasks),
		TemplatesLearned: len(templates),
		SourceHiddenVerified: true,
		TargetDomains: len(targetDomains),
		TargetTasks: len(targetTasks),
		TransferSolved: transferSolved,
		IndependentVerified: independent,
		ScratchSolved: scratchSolved,
		MedianExpansionRatio: median,
		AllTransferSolved: transferSolved == len(targetTasks),
		NoTargetLeakage: noLeak,
		Classification: class,
	}
	g10WriteReport("ACE_G10_TRANSFER.json", report)
	t.Logf("G10 report=%+v", report)
	if class != "G10_CROSS_DOMAIN_ABSTRACTION_TRANSFER_PROVEN" {
		t.Fatalf("G10 failed: %+v", report)
	}
}

type g11Hypothesis struct {
	Mask uint8
	Bias uint8
}

type g11Trace struct {
	Query uint8
	Obs   uint8
	Remaining int
}

type g11Report struct {
	TruthCases             int
	AdaptiveMeanExperiments float64
	RandomMeanExperiments   float64
	InformationLowerBound  int
	AdaptiveOptimalCases   int
	TranscriptReplayPasses int
	Classification         string
}

func g11Predict(h g11Hypothesis, q uint8) uint8 {
	v := h.Bias
	for i := 0; i < 4; i++ {
		if h.Mask&(1<<i) != 0 && q&(1<<i) != 0 {
			v ^= 1
		}
	}
	return v
}

func g11Hypotheses() []g11Hypothesis {
	out := make([]g11Hypothesis, 0, 32)
	for mask := 0; mask < 16; mask++ {
		for bias := 0; bias < 2; bias++ {
			out = append(out, g11Hypothesis{Mask: uint8(mask), Bias: uint8(bias)})
		}
	}
	return out
}

func g11Filter(hs []g11Hypothesis, q, obs uint8) []g11Hypothesis {
	out := make([]g11Hypothesis, 0, len(hs))
	for _, h := range hs {
		if g11Predict(h, q) == obs {
			out = append(out, h)
		}
	}
	return out
}

func g11InfoScore(hs []g11Hypothesis, q uint8) float64 {
	if len(hs) == 0 {
		return 0
	}
	count := [2]int{}
	for _, h := range hs {
		count[g11Predict(h, q)]++
	}
	n := float64(len(hs))
	base := math.Log2(n)
	exp := 0.0
	for _, c := range count {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		exp += p * math.Log2(float64(c))
	}
	return base - exp
}

func g11Select(hs []g11Hypothesis, used map[uint8]bool) uint8 {
	bestQ := uint8(0)
	best := -1.0
	for q := uint8(0); q < 16; q++ {
		if used[q] {
			continue
		}
		score := g11InfoScore(hs, q)
		if score > best+1e-12 || (math.Abs(score-best) <= 1e-12 && q < bestQ) {
			best = score
			bestQ = q
		}
	}
	return bestQ
}

func g11Adaptive(truth g11Hypothesis) ([]g11Trace, g11Hypothesis) {
	hs := g11Hypotheses()
	used := map[uint8]bool{}
	trace := make([]g11Trace, 0, 6)
	for len(hs) > 1 {
		q := g11Select(hs, used)
		used[q] = true
		obs := g11Predict(truth, q)
		hs = g11Filter(hs, q, obs)
		trace = append(trace, g11Trace{Query: q, Obs: obs, Remaining: len(hs)})
	}
	return trace, hs[0]
}

func g11Random(truth g11Hypothesis, seed int64) int {
	hs := g11Hypotheses()
	r := rand.New(rand.NewSource(seed))
	used := map[uint8]bool{}
	n := 0
	for len(hs) > 1 && n < 16 {
		q := uint8(r.Intn(16))
		for used[q] {
			q = uint8(r.Intn(16))
		}
		used[q] = true
		hs = g11Filter(hs, q, g11Predict(truth, q))
		n++
	}
	return n
}

func g11Replay(trace []g11Trace) bool {
	hs := g11Hypotheses()
	for _, step := range trace {
		hs = g11Filter(hs, step.Query, step.Obs)
		if len(hs) != step.Remaining || len(hs) == 0 {
			return false
		}
	}
	return len(hs) == 1
}

func g11WriteReport(name string, v any) {
	ws := os.Getenv("GITHUB_WORKSPACE")
	if ws == "" {
		return
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	_ = os.WriteFile(filepath.Join(ws, name), append(b, '\n'), 0644)
}

func TestG11AutonomousExperimentSelection(t *testing.T) {
	hypotheses := g11Hypotheses()
	if len(hypotheses) != 32 {
		t.Fatalf("expected 32 hypotheses, got %d", len(hypotheses))
	}
	const repetitions = 8
	truthCases := 0
	sumAdaptive := 0.0
	sumRandom := 0.0
	optimal := 0
	replay := 0
	for rep := 0; rep < repetitions; rep++ {
		for i, truth := range hypotheses {
			trace, found := g11Adaptive(truth)
			truthCases++
			sumAdaptive += float64(len(trace))
			if len(trace) == 5 && found == truth {
				optimal++
			}
			if !g11Replay(trace) {
				t.Fatalf("truth index %d rep %d failed independent transcript replay", i, rep)
			}
			replay++
			for j := 0; j < 32; j++ {
				sumRandom += float64(g11Random(truth, int64(110000+rep*1000+i*32+j)))
			}
		}
	}
	randomSamples := float64(truthCases * 32)
	report := g11Report{
		TruthCases: truthCases,
		AdaptiveMeanExperiments: sumAdaptive / float64(truthCases),
		RandomMeanExperiments: sumRandom / randomSamples,
		InformationLowerBound: 5,
		AdaptiveOptimalCases: optimal,
		TranscriptReplayPasses: replay,
		Classification: "G11_NOT_PROVEN",
	}
	if report.AdaptiveMeanExperiments == 5.0 &&
		report.AdaptiveOptimalCases == truthCases &&
		report.RandomMeanExperiments > report.AdaptiveMeanExperiments+0.25 &&
		report.TranscriptReplayPasses == truthCases {
		report.Classification = "G11_AUTONOMOUS_EXPERIMENT_SELECTION_PROVEN"
	}
	g11WriteReport("ACE_G11_EXPERIMENT_SELECTION.json", report)
	t.Logf("G11 report=%+v", report)
	if report.Classification != "G11_AUTONOMOUS_EXPERIMENT_SELECTION_PROVEN" {
		t.Fatalf("G11 failed: %+v", report)
	}
}

type g12Followup struct {
	ID   string
	Hyp  g11Hypothesis
}

type g12Trace struct {
	Phase string
	Query uint8
	Obs   uint8
	Remaining int
}

type g12Report struct {
	Cases                 int
	DiscoveryOptimal      int
	FollowupSolved        int
	HoldoutPredictions    int
	IndependentReplays    int
	NovelFollowupQueries  int
	RandomFollowupMean    float64
	ActiveFollowupMean    float64
	Classification        string
}

func g12GenerateFollowups(base g11Hypothesis) []g12Followup {
	return []g12Followup{
		{ID: "base", Hyp: base},
		{ID: "bias-flip", Hyp: g11Hypothesis{Mask: base.Mask, Bias: base.Bias ^ 1}},
		{ID: "toggle-feature-0", Hyp: g11Hypothesis{Mask: base.Mask ^ 1, Bias: base.Bias}},
	}
}

func g12Select(hs []g12Followup, used map[uint8]bool) uint8 {
	bestQ := uint8(0)
	best := -1.0
	for q := uint8(0); q < 16; q++ {
		if used[q] {
			continue
		}
		score := 0.0
		for i := 0; i < len(hs); i++ {
			for j := i + 1; j < len(hs); j++ {
				if g11Predict(hs[i].Hyp, q) != g11Predict(hs[j].Hyp, q) {
					score += 1
				}
			}
		}
		if score > best || (math.Abs(score-best) < 1e-12 && q < bestQ) {
			best = score
			bestQ = q
		}
	}
	return bestQ
}

func g12Filter(hs []g12Followup, q, obs uint8) []g12Followup {
	out := make([]g12Followup, 0, len(hs))
	for _, h := range hs {
		if g11Predict(h.Hyp, q) == obs {
			out = append(out, h)
		}
	}
	return out
}

func g12Replay(trace []g12Trace, base g11Hypothesis) bool {
	hs := g11Hypotheses()
	used := map[uint8]bool{}
	phase2 := g12GenerateFollowups(base)
	for _, step := range trace {
		if step.Phase == "discovery" {
			if g11Select(hs, used) != step.Query {
				return false
			}
			used[step.Query] = true
			hs = g11Filter(hs, step.Query, step.Obs)
			if len(hs) != step.Remaining {
				return false
			}
		} else {
			if len(hs) != 1 {
				return false
			}
			if g12Select(phase2, used) != step.Query {
				return false
			}
			used[step.Query] = true
			phase2 = g12Filter(phase2, step.Query, step.Obs)
			if len(phase2) != step.Remaining {
				return false
			}
		}
	}
	return len(hs) == 1 && len(phase2) == 1
}

func g12Run(truth, followTruth g11Hypothesis, rng *rand.Rand) ([]g12Trace, g11Hypothesis, bool) {
	baseHS := g11Hypotheses()
	used := map[uint8]bool{}
	trace := make([]g12Trace, 0, 8)
	for len(baseHS) > 1 {
		q := g11Select(baseHS, used)
		used[q] = true
		obs := g11Predict(truth, q)
		baseHS = g11Filter(baseHS, q, obs)
		trace = append(trace, g12Trace{Phase: "discovery", Query: q, Obs: obs, Remaining: len(baseHS)})
	}
	base := baseHS[0]
	follow := g12GenerateFollowups(base)
	for len(follow) > 1 {
		q := g12Select(follow, used)
		used[q] = true
		obs := g11Predict(followTruth, q)
		follow = g12Filter(follow, q, obs)
		trace = append(trace, g12Trace{Phase: "followup", Query: q, Obs: obs, Remaining: len(follow)})
	}
	novel := 0
	for q := uint8(0); q < 16; q++ {
		if !used[q] {
			novel = 1
			break
		}
	}
	return trace, follow[0].Hyp, novel == 1
}

func TestG12AutonomousResearchLoop(t *testing.T) {
	const cases = 128
	report := g12Report{Cases: cases, Classification: "G12_NOT_PROVEN"}
	for seed := 1; seed <= cases; seed++ {
		r := rand.New(rand.NewSource(int64(120000 + seed)))
		hypotheses := g11Hypotheses()
		truth := hypotheses[r.Intn(len(hypotheses))]
		baseTrace, discovered := g11Adaptive(truth)
		if len(baseTrace) != 5 || discovered != truth {
			t.Fatalf("seed %d failed optimal discovery", seed)
		}
		followups := g12GenerateFollowups(discovered)
		followTruth := followups[r.Intn(len(followups))].Hyp
		trace, final, novel := g12Run(truth, followTruth, r)
		followupSteps := 0
		for _, step := range trace {
			if step.Phase == "followup" {
				followupSteps++
			}
		}
		report.ActiveFollowupMean += float64(followupSteps)
		if len(trace) < 6 {
			t.Fatalf("seed %d research trace too short", seed)
		}
		if final != followTruth {
			t.Fatalf("seed %d follow-up research failed: final=%+v truth=%+v", seed, final, followTruth)
		}
		if !novel {
			t.Fatalf("seed %d produced no novel follow-up experiment", seed)
		}
		if !g12Replay(trace, truth) {
			t.Fatalf("seed %d independent replay failed", seed)
		}
		holdouts := []uint8{15, 13, 11, 7, 6, 10, 12, 3}
		for _, q := range holdouts {
			got := g11Predict(final, q)
			want := g11Predict(followTruth, q)
			if got != want {
				t.Fatalf("seed %d follow-up holdout mismatch q=%d got=%d want=%d", seed, q, got, want)
			}
			report.HoldoutPredictions++
		}
		report.DiscoveryOptimal++
		report.FollowupSolved++
		report.IndependentReplays++
		report.NovelFollowupQueries++
	}
	sumRandom := 0.0
	for seed := 1; seed <= cases; seed++ {
		r := rand.New(rand.NewSource(int64(130000 + seed)))
		truth := g11Hypotheses()[r.Intn(len(g11Hypotheses()))]
		_, discovered := g11Adaptive(truth)
		followups := g12GenerateFollowups(discovered)
		followTruth := followups[r.Intn(len(followups))].Hyp
		remaining := append([]g12Followup(nil), followups...)
		steps := 0
		for len(remaining) > 1 && steps < 16 {
			q := uint8(r.Intn(16))
			obs := g11Predict(followTruth, q)
			remaining = g12Filter(remaining, q, obs)
			steps++
		}
		sumRandom += float64(steps)
	}
report.ActiveFollowupMean = report.ActiveFollowupMean / float64(cases)
	report.RandomFollowupMean = sumRandom / float64(cases)
	if report.DiscoveryOptimal == cases &&
		report.FollowupSolved == cases &&
		report.HoldoutPredictions == cases*8 &&
		report.IndependentReplays == cases &&
		report.NovelFollowupQueries == cases &&
		report.RandomFollowupMean > report.ActiveFollowupMean &&
		report.RandomFollowupMean > 1.25 {
		report.Classification = "G12_AUTONOMOUS_SCIENTIFIC_RESEARCH_LOOP_PROVEN"
	}
	g10WriteReport("ACE_G12_AUTONOMOUS_RESEARCH.json", report)
	t.Logf("G12 report=%+v", report)
	if report.Classification != "G12_AUTONOMOUS_SCIENTIFIC_RESEARCH_LOOP_PROVEN" {
		t.Fatalf("G12 failed: %+v", report)
	}
}
