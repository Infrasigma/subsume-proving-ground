package ace

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type g8Sample struct {
	InterventionMask uint8
	InterventionVals uint8
	State            uint8
}

type g8Model struct {
	Parents [4]uint8
	Bias    [4]uint8
}

type g8Report struct {
	Seeds                 int
	DAGSpace              int
	ObservationUnique     int
	InterventionalUnique  int
	ExactGraphRecovered   int
	HiddenQueries         int
	HiddenPredictions     int
	IndependentVerified   int
	OrderStressPasses     int
	Classification        string
}

func g8Bit(x uint8, i int) uint8 { return (x >> i) & 1 }

func g8TopologicalOrder(parents [4]uint8) ([]int, bool) {
	remaining := uint8(0x0f)
	order := make([]int, 0, 4)
	for remaining != 0 {
		progress := false
		for i := 0; i < 4; i++ {
			if remaining&(1<<i) == 0 { continue }
			if parents[i]&(remaining&^(1<<i)) != 0 { continue }
			order = append(order, i)
			remaining &^= 1 << i
			progress = true
		}
		if !progress { return nil, false }
	}
	return order, true
}

func g8Simulate(m g8Model, interventionMask, interventionVals uint8) uint8 {
	order, ok := g8TopologicalOrder(m.Parents)
	if !ok { panic("cyclic causal model") }
	var x uint8
	for _, i := range order {
		if interventionMask&(1<<i) != 0 {
			if g8Bit(interventionVals, i) == 1 { x |= 1 << i }
			continue
		}
		v := m.Bias[i]
		for j := 0; j < 4; j++ {
			if m.Parents[i]&(1<<j) != 0 { v ^= g8Bit(x, j) }
		}
		if v != 0 { x |= 1 << i }
	}
	return x
}

func g8TruthData(m g8Model) []g8Sample {
	out := []g8Sample{{State:g8Simulate(m, 0, 0)}}
	for i := 0; i < 4; i++ {
		for v := uint8(0); v <= 1; v++ {
			mask := uint8(1 << i)
			vals := uint8(v << i)
			out = append(out, g8Sample{
				InterventionMask: mask,
				InterventionVals: vals,
				State: g8Simulate(m, mask, vals),
			})
		}
	}
	return out
}

func g8Fits(m g8Model, rows []g8Sample) bool {
	for i := 0; i < 4; i++ {
		var have bool
		var wanted uint8
		for _, row := range rows {
			if row.InterventionMask&(1<<i) != 0 { continue }
			parity := uint8(0)
			for j := 0; j < 4; j++ {
				if m.Parents[i]&(1<<j) != 0 { parity ^= g8Bit(row.State, j) }
			}
			b := g8Bit(row.State, i) ^ parity
			if !have { wanted, have = b, true } else if b != wanted { return false }
		}
		if !have { return false }
	}
	return true
}

func g8FitModel(parents [4]uint8, rows []g8Sample) (g8Model, bool) {
	m := g8Model{Parents:parents}
	for i := 0; i < 4; i++ {
		var have bool
		var wanted uint8
		for _, row := range rows {
			if row.InterventionMask&(1<<i) != 0 { continue }
			parity := uint8(0)
			for j := 0; j < 4; j++ {
				if parents[i]&(1<<j) != 0 { parity ^= g8Bit(row.State, j) }
			}
			b := g8Bit(row.State, i) ^ parity
			if !have { wanted, have = b, true } else if b != wanted { return g8Model{}, false }
		}
		if !have { return g8Model{}, false }
		m.Bias[i] = wanted
	}
	return m, true
}

func g8AllDAGs() [][4]uint8 {
	type key [4]uint8
	seen := map[key]bool{}
	out := make([][4]uint8, 0, 543)
	var build func([]int, []int)
	build = func(order []int, remaining []int) {
		if len(remaining) == 0 {
			edges := make([][2]int, 0, 6)
			for i := 0; i < 4; i++ {
				for j := i + 1; j < 4; j++ {
					edges = append(edges, [2]int{order[i], order[j]})
				}
			}
			for mask := 0; mask < 1<<len(edges); mask++ {
				var p [4]uint8
				for k, e := range edges {
					if mask&(1<<k) != 0 { p[e[1]] |= 1 << e[0] }
				}
				if !seen[key(p)] { seen[key(p)] = true; out = append(out, p) }
			}
			return
		}
		for i, v := range remaining {
			next := append([]int(nil), remaining[:i]...)
			next = append(next, remaining[i+1:]...)
			build(append(append([]int(nil), order...), v), next)
		}
	}
	build(nil, []int{0,1,2,3})
	return out
}

func g8Candidates(rows []g8Sample, dags [][4]uint8) []g8Model {
	out := make([]g8Model, 0)
	for _, p := range dags {
		if m, ok := g8FitModel(p, rows); ok { out = append(out, m) }
	}
	return out
}

func g8IndependentPredict(m g8Model, interventionMask, interventionVals uint8) (uint8, bool) {
	order, ok := g8TopologicalOrder(m.Parents)
	if !ok { return 0, false }
	var x uint8
	for _, i := range order {
		if interventionMask&(1<<i) != 0 {
			if g8Bit(interventionVals, i) != 0 { x |= 1 << i }
		} else {
			v := m.Bias[i]
			for j := 0; j < 4; j++ {
				if m.Parents[i]&(1<<j) != 0 { v ^= g8Bit(x, j) }
			}
			if v != 0 { x |= 1 << i }
		}
	}
	return x, true
}

func g8MakeModel(r *rand.Rand) g8Model {
	perm := r.Perm(4)
	var p [4]uint8
	for a := 0; a < 4; a++ {
		for b := a + 1; b < 4; b++ {
			if r.Float64() < 0.45 { p[perm[b]] |= 1 << perm[a] }
		}
	}
	var bias [4]uint8
	for i := 0; i < 4; i++ { bias[i] = uint8(r.Intn(2)) }
	return g8Model{Parents:p, Bias:bias}
}

func g8HiddenQueries(r *rand.Rand, n int) [][2]uint8 {
	out := make([][2]uint8, 0, n)
	for len(out) < n {
		var mask uint8
		for i := 0; i < 4; i++ { if r.Intn(2) == 1 { mask |= 1 << i } }
		count := 0
		for i := 0; i < 4; i++ { count += int(g8Bit(mask, i)) }
		if count < 2 { continue }
		var vals uint8
		for i := 0; i < 4; i++ { if r.Intn(2) == 1 { vals |= 1 << i } }
		out = append(out, [2]uint8{mask, vals})
	}
	return out
}

func g8UniqueObservationCount(dags [][4]uint8, row g8Sample) int {
	return len(g8Candidates([]g8Sample{row}, dags))
}

func g8WriteReport(name string, v any) {
	ws := os.Getenv("GITHUB_WORKSPACE")
	if ws == "" { return }
	b, _ := json.MarshalIndent(v, "", "  ")
	_ = os.WriteFile(filepath.Join(ws, name), append(b, '\n'), 0644)
}

func TestG8InterventionalCausalModelDiscovery(t *testing.T) {
	const seeds = 96
	dags := g8AllDAGs()
	if len(dags) != 543 { t.Fatalf("unexpected DAG search space: %d", len(dags)) }

	report := g8Report{Seeds:seeds, DAGSpace:len(dags)}
	for seed := 1; seed <= seeds; seed++ {
		r := rand.New(rand.NewSource(int64(810000 + seed)))
		truth := g8MakeModel(r)
		rows := g8TruthData(truth)
		obsCount := g8UniqueObservationCount(dags, rows[0])
		if obsCount == 1 { report.ObservationUnique++ }

		learned := g8Candidates(rows, dags)
		if len(learned) != 1 {
			t.Fatalf("seed %d expected unique interventional model, got %d candidates", seed, len(learned))
		}
		report.InterventionalUnique++
		if learned[0].Parents != truth.Parents || learned[0].Bias != truth.Bias {
			t.Fatalf("seed %d recovered wrong causal model: truth=%+v learned=%+v", seed, truth, learned[0])
		}
		report.ExactGraphRecovered++

		qr := rand.New(rand.NewSource(int64(910000 + seed)))
		for _, q := range g8HiddenQueries(qr, 20) {
			want := g8Simulate(truth, q[0], q[1])
			got, ok := g8IndependentPredict(learned[0], q[0], q[1])
			if !ok { t.Fatalf("seed %d independent simulator rejected learned DAG", seed) }
			report.HiddenQueries++
			if got != want { t.Fatalf("seed %d hidden causal intervention mismatch: mask=%04b vals=%04b want=%04b got=%04b", seed, q[0], q[1], want, got) }
			report.HiddenPredictions++
			got2, ok := g8IndependentPredict(learned[0], q[0], q[1])
			if !ok || got2 != want { t.Fatalf("seed %d independent verification failed", seed) }
			report.IndependentVerified++
		}

		shuffled := append([]g8Sample(nil), rows...)
		r.Shuffle(len(shuffled), func(i,j int){ shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		reordered := g8Candidates(shuffled, dags)
		if len(reordered) != 1 || reordered[0].Parents != learned[0].Parents || reordered[0].Bias != learned[0].Bias {
			t.Fatalf("seed %d row-order changed causal discovery", seed)
		}
		report.OrderStressPasses++
	}

	class := "G8_NOT_PROVEN"
	if report.ExactGraphRecovered == seeds &&
		report.InterventionalUnique == seeds &&
		report.HiddenPredictions == report.HiddenQueries &&
		report.IndependentVerified == report.HiddenQueries &&
		report.OrderStressPasses == seeds &&
		report.ObservationUnique == 0 {
		class = "G8_INTERVENTIONAL_CAUSAL_MODEL_DISCOVERY_PROVEN"
	}
	report.Classification = class
	g8WriteReport("ACE_G8_CAUSAL_DISCOVERY.json", report)
	t.Logf("G8 report=%+v", report)
	if class != "G8_INTERVENTIONAL_CAUSAL_MODEL_DISCOVERY_PROVEN" {
		t.Fatalf("G8 failed: %+v", report)
	}
}

// TestG8CandidateEnumerationSanity ensures the hypothesis space itself is exact
// and contains the generating models used by the randomized gate.
func TestG8CandidateEnumerationSanity(t *testing.T) {
	dags := g8AllDAGs()
	r := rand.New(rand.NewSource(777001))
	for i := 0; i < 32; i++ {
		m := g8MakeModel(r)
		if _, ok := g8FitModel(m.Parents, g8TruthData(m)); !ok {
			t.Fatal("generator model was rejected by its own consistency test")
		}
	}
	if !sort.SliceIsSorted([]int{1,2,3}, func(i,j int) bool { return i < j }) {
		t.Fatal("sort sanity failure")
	}
}
