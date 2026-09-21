
package ace

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type g4Kind string

const (
	g4Raw g4Kind = "raw"
	g4Compressed g4Kind = "compressed"
	g4Verified g4Kind = "verified"
	g4Failure g4Kind = "failure"
	g4Reconstruction g4Kind = "reconstruction"
)

var g4Kinds = []g4Kind{g4Raw, g4Compressed, g4Verified, g4Failure, g4Reconstruction}

type g4Task struct{ Key, Mode int }
type g4Memory struct {
	Key   int
	Kind  g4Kind
	Bytes int
}
type g4Report struct {
	Seeds int
	MeanAdaptive float64
	MeanBestFixed float64
	MeanRandom float64
	MeanOracle float64
	OracleFraction float64
	AllBeatFixed bool
	AllOracleBand bool
	SelectedKinds []string
	FutureUntouched bool
	Classification string
}

func g4Bytes(k g4Kind) int {
	switch k {
	case g4Raw: return 8
	case g4Compressed: return 2
	case g4Verified: return 3
	case g4Failure: return 1
	case g4Reconstruction: return 4
	default: panic("unknown memory kind")
	}
}

func g4Savings(m g4Memory, t g4Task) int {
	if m.Key != t.Key { return 0 }
	switch t.Mode {
	case 0:
		switch m.Kind {
		case g4Raw: return 6
		case g4Compressed: return 12
		case g4Verified: return 16
		case g4Reconstruction: return 4
		}
	case 1:
		switch m.Kind {
		case g4Raw: return 2
		case g4Compressed: return 5
		case g4Verified: return 12
		case g4Failure: return 1
		case g4Reconstruction: return 8
		}
	case 2:
		switch m.Kind {
		case g4Raw: return 1
		case g4Compressed: return 2
		case g4Verified: return 3
		case g4Failure: return 14
		case g4Reconstruction: return 2
		}
	case 3:
		switch m.Kind {
		case g4Raw: return 5
		case g4Compressed: return 4
		case g4Verified: return 6
		case g4Failure: return 1
		case g4Reconstruction: return 14
		}
	}
	return 0
}

func g4Weights(key int) [4]int {
	all := [][4]int{
		{34,1,1,1}, {2,34,2,2}, {2,3,34,2},
		{2,2,2,34}, {18,18,4,4}, {22,4,3,17},
	}
	return all[key%len(all)]
}

func g4Draw(r *rand.Rand, key int) g4Task {
	w := g4Weights(key)
	n := 0
	for _, x := range w { n += x }
	v := r.Intn(n)
	for mode, x := range w {
		if v < x { return g4Task{Key:key, Mode:mode} }
		v -= x
	}
	return g4Task{Key:key, Mode:0}
}

func g4Candidates() []g4Memory {
	out := make([]g4Memory, 0, 30)
	for k := 0; k < 6; k++ {
		for _, kind := range g4Kinds {
			out = append(out, g4Memory{Key:k, Kind:kind, Bytes:g4Bytes(kind)})
		}
	}
	return out
}

func g4PredictedSavings(c g4Memory, observed []g4Task) float64 {
	if len(observed) == 0 { return 0 }
	total := 0
	for _, t := range observed { total += g4Savings(c, t) }
	return float64(total+1) / float64(len(observed)+2)
}

func g4Select(pool []g4Memory, observed []g4Task, budget int) []g4Memory {
	groups := map[int][]g4Memory{}
	for _, c := range pool {
		groups[c.Key] = append(groups[c.Key], c)
	}
	keys := make([]int, 0, len(groups))
	for k := range groups { keys = append(keys, k) }
	sort.Ints(keys)

	dp := make([]float64, budget+1)
	choice := make([][]g4Memory, budget+1)
	for i := range dp { dp[i] = -math.MaxFloat64 }
	dp[0] = 0

	for _, key := range keys {
		next := make([]float64, budget+1)
		nextChoice := make([][]g4Memory, budget+1)
		for i := range next { next[i] = -math.MaxFloat64 }
		for b := 0; b <= budget; b++ {
			if dp[b] == -math.MaxFloat64 { continue }
			if dp[b] > next[b] {
				next[b] = dp[b]
				nextChoice[b] = append([]g4Memory(nil), choice[b]...)
			}
			for _, c := range groups[key] {
				if b+c.Bytes > budget { continue }
				value := g4PredictedSavings(c, observed) / float64(c.Bytes)
				v := dp[b] + value
				if v > next[b+c.Bytes] {
					next[b+c.Bytes] = v
					nextChoice[b+c.Bytes] = append(append([]g4Memory(nil), choice[b]...), c)
				}
			}
		}
		dp, choice = next, nextChoice
	}
	best := 0
	for b := 1; b <= budget; b++ {
		if dp[b] > dp[best] { best = b }
	}
	return choice[best]
}
func g4Actual(sel []g4Memory, future []g4Task) int {
	byKey := map[int]g4Memory{}
	for _, c := range sel { byKey[c.Key] = c }
	total := 0
	for _, t := range future {
		if c, ok := byKey[t.Key]; ok { total += g4Savings(c, t) }
	}
	return total
}

func g4Fixed(pool []g4Memory, observed, future []g4Task, kind g4Kind, budget int) int {
	filtered := make([]g4Memory, 0, 6)
	for _, c := range pool { if c.Kind == kind { filtered = append(filtered, c) } }
	return g4Actual(g4Select(filtered, observed, budget), future)
}

func g4Random(pool []g4Memory, future []g4Task, seed int, budget int) int {
	r := rand.New(rand.NewSource(int64(seed)))
	p := append([]g4Memory(nil), pool...)
	r.Shuffle(len(p), func(i,j int){ p[i],p[j] = p[j],p[i] })
	used := 0
	seen := map[int]bool{}
	sel := make([]g4Memory,0)
	for _, c := range p {
		if seen[c.Key] || used+c.Bytes > budget { continue }
		seen[c.Key] = true
		used += c.Bytes
		sel = append(sel, c)
	}
	return g4Actual(sel, future)
}

func g4Oracle(pool []g4Memory, future []g4Task, budget int) int {
	return g4Actual(g4Select(pool, future, budget), future)
}

func g4WriteReport(name string, v any) {
	ws := os.Getenv("GITHUB_WORKSPACE")
	if ws == "" { return }
	b, _ := json.MarshalIndent(v, "", "  ")
	_ = os.WriteFile(filepath.Join(ws, name), append(b, '
'), 0644)
}

func TestG4AutonomousMemorySelection(t *testing.T) {
	const seeds = 32
	const budget = 16
	pool := g4Candidates()
	var adaptive, fixed, randomTotal, oracle float64
	allBeat, allBand := true, true
	futureUntouched := true
	kindSeen := map[g4Kind]bool{}

	for seed := 1; seed <= seeds; seed++ {
		r := rand.New(rand.NewSource(int64(700000 + seed)))
		observed := make([]g4Task,0,72)
		for i := 0; i < 72; i++ { observed = append(observed, g4Draw(r, i%6)) }
		future := make([]g4Task,0,144)
		for i := 0; i < 144; i++ { future = append(future, g4Draw(r, (i+seed)%6)) }

		futureSnapshot := append([]g4Task(nil), future...)
		sel := g4Select(pool, observed, budget)
		if !equalG4Tasks(future, futureSnapshot) { futureUntouched = false }

		a := g4Actual(sel, future)
		o := g4Oracle(pool, future, budget)
		bestFixed := 0
		for _, kind := range g4Kinds {
			if x := g4Fixed(pool, observed, future, kind, budget); x > bestFixed { bestFixed = x }
		}
		rnd := g4Random(pool, future, seed, budget)
		adaptive += float64(a)
		fixed += float64(bestFixed)
		randomTotal += float64(rnd)
		oracle += float64(o)
		if a <= bestFixed { allBeat = false }
		if o > 0 && float64(a)/float64(o) < 0.80 { allBand = false }
		for _, c := range sel { kindSeen[c.Kind] = true }
	}
	if oracle == 0 { t.Fatal("oracle utility is zero") }

	kinds := make([]string,0,len(kindSeen))
	for k := range kindSeen { kinds = append(kinds,string(k)) }
	sort.Strings(kinds)

	r := g4Report{
		Seeds: seeds,
		MeanAdaptive: adaptive/seeds,
		MeanBestFixed: fixed/seeds,
		MeanRandom: randomTotal/seeds,
		MeanOracle: oracle/seeds,
		OracleFraction: adaptive/oracle,
		AllBeatFixed: allBeat,
		AllOracleBand: allBand,
		SelectedKinds: kinds,
		FutureUntouched: futureUntouched,
		Classification: "G4_NOT_PROVEN",
	}
	if allBeat && allBand && futureUntouched && len(kinds) >= 3 {
		r.Classification = "G4_AUTONOMOUS_MEMORY_SELECTION_PROVEN"
	}
	g4WriteReport("ACE_G4_MEMORY_SELECTION.json", r)
	t.Logf("G4 report=%+v", r)
	if r.Classification != "G4_AUTONOMOUS_MEMORY_SELECTION_PROVEN" {
		t.Fatalf("G4 failed: %+v", r)
	}
}

func equalG4Tasks(a,b []g4Task) bool {
	if len(a) != len(b) { return false }
	for i := range a {
		if a[i] != b[i] { return false }
	}
	return true
}
